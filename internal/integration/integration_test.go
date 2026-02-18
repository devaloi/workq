package integration

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devaloi/workq/internal/deadletter"
	"github.com/devaloi/workq/internal/domain"
	"github.com/devaloi/workq/internal/handler"
	"github.com/devaloi/workq/internal/queue"
	"github.com/devaloi/workq/internal/retry"
	"github.com/devaloi/workq/internal/worker"
)

func TestIntegration_AllJobsCompleteOrDead(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var completed atomic.Int32
	reg := handler.NewRegistry()
	_ = reg.Register("work", func(_ context.Context, _ []byte) error {
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
		if rand.Float64() < 0.3 {
			return fmt.Errorf("transient error")
		}
		completed.Add(1)
		return nil
	})

	backoff := &retry.Backoff{Base: 1 * time.Millisecond, Max: 10 * time.Millisecond, JitterMax: 1 * time.Millisecond}
	dl := deadletter.NewStore()
	proc := worker.NewProcessor(mq, reg, backoff, dl)
	pool := worker.NewPool(proc, 5)

	ctx := context.Background()
	const jobCount = 50
	for i := 0; i < jobCount; i++ {
		j, _ := domain.NewJob("work", nil, 5)
		j.Priority = rand.Intn(10)
		_ = mq.Enqueue(ctx, j)
	}

	pool.Start(ctx)

	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timeout: not all jobs completed")
		case <-ticker.C:
			stats, _ := mq.Stats(ctx)
			if stats.Pending == 0 && stats.Active == 0 {
				pool.Shutdown(5 * time.Second)
				total := int(completed.Load()) + dl.Len()
				if total != jobCount {
					t.Fatalf("expected %d total (completed + dead), got %d (completed=%d, dead=%d)",
						jobCount, total, completed.Load(), dl.Len())
				}
				t.Logf("completed=%d, dead=%d, failed_attempts=%d",
					completed.Load(), dl.Len(), stats.Failed)
				return
			}
		}
	}
}

func TestIntegration_GracefulShutdownMidProcessing(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var completed atomic.Int32
	reg := handler.NewRegistry()
	_ = reg.Register("slow", func(ctx context.Context, _ []byte) error {
		select {
		case <-time.After(200 * time.Millisecond):
			completed.Add(1)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	backoff := retry.DefaultBackoff()
	proc := worker.NewProcessor(mq, reg, backoff, nil)
	pool := worker.NewPool(proc, 3)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		j, _ := domain.NewJob("slow", nil, 1)
		_ = mq.Enqueue(ctx, j)
	}

	pool.Start(ctx)
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	pool.Shutdown(5 * time.Second)
	elapsed := time.Since(start)

	if elapsed > 6*time.Second {
		t.Fatalf("shutdown too slow: %v", elapsed)
	}

	c := completed.Load()
	t.Logf("completed %d/10 jobs before shutdown (%v)", c, elapsed)
	if c == 0 {
		t.Fatal("expected at least some jobs to complete")
	}
}

func TestIntegration_PriorityOrdering(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	var orderMu sync.Mutex
	var order []string

	reg := handler.NewRegistry()
	_ = reg.Register("ordered", func(_ context.Context, payload []byte) error {
		orderMu.Lock()
		order = append(order, string(payload))
		orderMu.Unlock()
		return nil
	})

	backoff := retry.DefaultBackoff()
	proc := worker.NewProcessor(mq, reg, backoff, nil)
	pool := worker.NewPool(proc, 1)

	ctx := context.Background()
	for i := 5; i >= 1; i-- {
		j, _ := domain.NewJob("ordered", []byte(fmt.Sprintf("p%d", i)), 1)
		j.Priority = i
		_ = mq.Enqueue(ctx, j)
	}

	pool.Start(ctx)
	time.Sleep(1 * time.Second)
	pool.Shutdown(5 * time.Second)

	orderMu.Lock()
	orderCopy := make([]string, len(order))
	copy(orderCopy, order)
	orderMu.Unlock()
	if len(orderCopy) != 5 {
		t.Fatalf("expected 5 processed, got %d", len(orderCopy))
	}

	for i, got := range orderCopy {
		expected := fmt.Sprintf("p%d", i+1)
		if got != expected {
			t.Fatalf("position %d: expected %s, got %s (order: %v)", i, expected, got, orderCopy)
		}
	}
}

func TestIntegration_PersistentQueueRecovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/state.json"
	ctx := context.Background()

	pq1, _ := queue.NewPersistentQueue(path)
	for i := 0; i < 10; i++ {
		j, _ := domain.NewJob("persist_test", []byte(fmt.Sprintf("job_%d", i)), 3)
		_ = pq1.Enqueue(ctx, j)
	}
	for i := 0; i < 3; i++ {
		_, _ = pq1.Dequeue(ctx)
	}
	pq1.Close()

	pq2, err := queue.NewPersistentQueue(path)
	if err != nil {
		t.Fatalf("recovery: %v", err)
	}
	defer pq2.Close()

	stats, _ := pq2.Stats(ctx)
	if stats.Pending != 10 {
		t.Fatalf("expected 10 recovered pending, got %d", stats.Pending)
	}
}
