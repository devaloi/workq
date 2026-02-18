package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devaloi/workq/internal/domain"
	"github.com/devaloi/workq/internal/handler"
	"github.com/devaloi/workq/internal/queue"
	"github.com/devaloi/workq/internal/retry"
)

func TestPool_ProcessesJobs(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var processed atomic.Int32
	reg := handler.NewRegistry()
	_ = reg.Register("count", func(_ context.Context, _ []byte) error {
		processed.Add(1)
		return nil
	})

	b := retry.DefaultBackoff()
	proc := NewProcessor(mq, reg, b, nil)
	pool := NewPool(proc, 4)

	ctx := context.Background()
	for i := 0; i < 20; i++ {
		j, _ := domain.NewJob("count", nil, 1)
		_ = mq.Enqueue(ctx, j)
	}

	pool.Start(ctx)
	time.Sleep(500 * time.Millisecond)
	pool.Shutdown(5 * time.Second)

	if got := processed.Load(); got != 20 {
		t.Fatalf("expected 20 processed, got %d", got)
	}
}

func TestPool_RespectsContextCancellation(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	_ = reg.Register("slow", func(ctx context.Context, _ []byte) error {
		select {
		case <-time.After(10 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	b := retry.DefaultBackoff()
	proc := NewProcessor(mq, reg, b, nil)
	pool := NewPool(proc, 2)

	ctx := context.Background()
	j, _ := domain.NewJob("slow", nil, 1)
	_ = mq.Enqueue(ctx, j)

	pool.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	pool.Shutdown(2 * time.Second)
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Fatalf("shutdown took too long: %v", elapsed)
	}
}

func TestPool_ConcurrencyLimit(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	reg := handler.NewRegistry()
	_ = reg.Register("track", func(_ context.Context, _ []byte) error {
		cur := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	})

	b := retry.DefaultBackoff()
	proc := NewProcessor(mq, reg, b, nil)
	pool := NewPool(proc, 3)

	ctx := context.Background()
	for i := 0; i < 30; i++ {
		j, _ := domain.NewJob("track", nil, 1)
		_ = mq.Enqueue(ctx, j)
	}

	pool.Start(ctx)
	time.Sleep(2 * time.Second)
	pool.Shutdown(5 * time.Second)

	if max := maxConcurrent.Load(); max > 3 {
		t.Fatalf("max concurrent %d exceeded pool size 3", max)
	}
}

func TestPool_GracefulShutdown(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var completed atomic.Int32
	reg := handler.NewRegistry()
	_ = reg.Register("work", func(_ context.Context, _ []byte) error {
		time.Sleep(100 * time.Millisecond)
		completed.Add(1)
		return nil
	})

	b := retry.DefaultBackoff()
	dl := &mockDeadLetter{}
	_ = dl
	proc := NewProcessor(mq, reg, b, nil)
	pool := NewPool(proc, 2)

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		j, _ := domain.NewJob("work", nil, 1)
		_ = mq.Enqueue(ctx, j)
	}

	pool.Start(ctx)
	time.Sleep(50 * time.Millisecond) // let workers pick up jobs
	pool.Shutdown(5 * time.Second)

	// In-flight jobs should have completed.
	if c := completed.Load(); c == 0 {
		t.Fatal("expected some jobs to complete during graceful shutdown")
	}
}

func TestPool_FailedJobsRetry(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var attempts atomic.Int32
	reg := handler.NewRegistry()
	_ = reg.Register("flaky", func(_ context.Context, _ []byte) error {
		if attempts.Add(1) <= 2 {
			return fmt.Errorf("transient error")
		}
		return nil
	})

	b := &retry.Backoff{Base: 0, Max: 0, JitterMax: 0}
	proc := NewProcessor(mq, reg, b, nil)
	pool := NewPool(proc, 1)

	ctx := context.Background()
	j, _ := domain.NewJob("flaky", nil, 5)
	_ = mq.Enqueue(ctx, j)

	pool.Start(ctx)
	time.Sleep(1 * time.Second)
	pool.Shutdown(5 * time.Second)

	stats, _ := mq.Stats(ctx)
	if stats.Completed != 1 {
		t.Fatalf("expected 1 completed after retries, got %d (attempts: %d)", stats.Completed, attempts.Load())
	}
}
