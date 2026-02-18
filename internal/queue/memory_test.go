package queue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/devaloi/workq/internal/domain"
)

func TestMemoryQueue_EnqueueDequeue(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()

	job, _ := domain.NewJob("test", []byte("payload"), 3)
	ctx := context.Background()

	if err := mq.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, err := mq.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("expected job %s, got %s", job.ID, got.ID)
	}
	if got.Status != domain.StatusActive {
		t.Fatalf("expected active status, got %s", got.Status)
	}
}

func TestMemoryQueue_PriorityOrdering(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()
	ctx := context.Background()

	// Enqueue low priority first, then high priority.
	low, _ := domain.NewJob("low", nil, 1)
	low.Priority = 10

	high, _ := domain.NewJob("high", nil, 1)
	high.Priority = 1

	_ = mq.Enqueue(ctx, low)
	_ = mq.Enqueue(ctx, high)

	// High priority should dequeue first.
	got, _ := mq.Dequeue(ctx)
	if got.ID != high.ID {
		t.Fatalf("expected high priority job first, got %s (priority %d)", got.Type, got.Priority)
	}
}

func TestMemoryQueue_DelayedScheduling(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()
	ctx := context.Background()

	job, _ := domain.NewJob("delayed", nil, 1)
	job.ScheduledAt = time.Now().Add(100 * time.Millisecond)
	_ = mq.Enqueue(ctx, job)

	// Should not be immediately available — add a ready job.
	ready, _ := domain.NewJob("ready", nil, 1)
	ready.Priority = 10 // lower priority but ready now
	_ = mq.Enqueue(ctx, ready)

	got, _ := mq.Dequeue(ctx)
	if got.ID != ready.ID {
		t.Fatalf("expected ready job, got %s", got.Type)
	}
}

func TestMemoryQueue_Ack(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()
	ctx := context.Background()

	job, _ := domain.NewJob("test", nil, 1)
	_ = mq.Enqueue(ctx, job)
	got, _ := mq.Dequeue(ctx)

	if err := mq.Ack(ctx, got.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}

	stats, _ := mq.Stats(ctx)
	if stats.Completed != 1 {
		t.Fatalf("expected 1 completed, got %d", stats.Completed)
	}
}

func TestMemoryQueue_FailRetry(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()
	ctx := context.Background()

	job, _ := domain.NewJob("test", nil, 3)
	_ = mq.Enqueue(ctx, job)
	got, _ := mq.Dequeue(ctx)

	if err := mq.Fail(ctx, got.ID, fmt.Errorf("oops")); err != nil {
		t.Fatalf("fail: %v", err)
	}

	stats, _ := mq.Stats(ctx)
	if stats.Pending != 1 {
		t.Fatalf("expected 1 pending (re-enqueued), got %d", stats.Pending)
	}
}

func TestMemoryQueue_FailDeadLetter(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()
	ctx := context.Background()

	job, _ := domain.NewJob("test", nil, 1)
	_ = mq.Enqueue(ctx, job)
	got, _ := mq.Dequeue(ctx)

	if err := mq.Fail(ctx, got.ID, fmt.Errorf("fatal")); err != nil {
		t.Fatalf("fail: %v", err)
	}

	stats, _ := mq.Stats(ctx)
	if stats.Dead != 1 {
		t.Fatalf("expected 1 dead, got %d", stats.Dead)
	}
}

func TestMemoryQueue_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup

	// Enqueue concurrently.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, _ := domain.NewJob("concurrent", nil, 1)
			_ = mq.Enqueue(ctx, j)
		}()
	}
	wg.Wait()

	// Dequeue concurrently.
	var dequeued int
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := mq.Dequeue(ctx)
			if err == nil && j != nil {
				mu.Lock()
				dequeued++
				mu.Unlock()
				_ = mq.Ack(ctx, j.ID)
			}
		}()
	}
	wg.Wait()

	if dequeued != n {
		t.Fatalf("expected %d dequeued, got %d", n, dequeued)
	}
}

func TestMemoryQueue_DequeueContextCancel(t *testing.T) {
	t.Parallel()

	mq := NewMemoryQueue()
	defer mq.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := mq.Dequeue(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
