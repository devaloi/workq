package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/devaloi/workq/internal/domain"
	"github.com/devaloi/workq/internal/handler"
	"github.com/devaloi/workq/internal/queue"
	"github.com/devaloi/workq/internal/retry"
)

type mockDeadLetter struct {
	mu   sync.Mutex
	jobs []*domain.Job
}

func (m *mockDeadLetter) Add(job *domain.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
}

func (m *mockDeadLetter) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.jobs)
}

func TestProcessor_SuccessfulJob(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	_ = reg.Register("test", func(_ context.Context, _ []byte) error {
		return nil
	})

	p := NewProcessor(mq, reg, nil, nil)
	ctx := context.Background()

	job, _ := domain.NewJob("test", []byte("data"), 3)
	_ = mq.Enqueue(ctx, job)

	if ok := p.Process(ctx); !ok {
		t.Fatal("expected Process to succeed")
	}

	stats, _ := mq.Stats(ctx)
	if stats.Completed != 1 {
		t.Fatalf("expected 1 completed, got %d", stats.Completed)
	}
}

func TestProcessor_FailedJobWithRetry(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	_ = reg.Register("fail", func(_ context.Context, _ []byte) error {
		return fmt.Errorf("temporary error")
	})

	b := &retry.Backoff{Base: 0, Max: 0, JitterMax: 0}
	p := NewProcessor(mq, reg, b, nil)
	ctx := context.Background()

	job, _ := domain.NewJob("fail", nil, 3) // 3 max attempts
	_ = mq.Enqueue(ctx, job)

	p.Process(ctx)

	stats, _ := mq.Stats(ctx)
	if stats.Pending != 1 {
		t.Fatalf("expected 1 pending (re-enqueued), got %d", stats.Pending)
	}
}

func TestProcessor_DeadLetterAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	_ = reg.Register("fail", func(_ context.Context, _ []byte) error {
		return fmt.Errorf("permanent error")
	})

	dl := &mockDeadLetter{}
	b := &retry.Backoff{Base: 0, Max: 0, JitterMax: 0}
	p := NewProcessor(mq, reg, b, dl)
	ctx := context.Background()

	job, _ := domain.NewJob("fail", nil, 1) // 1 max attempt
	_ = mq.Enqueue(ctx, job)

	p.Process(ctx)

	stats, _ := mq.Stats(ctx)
	if stats.Dead != 1 {
		t.Fatalf("expected 1 dead, got %d", stats.Dead)
	}
	if dl.Len() != 1 {
		t.Fatalf("expected 1 in dead letter store, got %d", dl.Len())
	}
}

func TestProcessor_PanicRecovery(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	_ = reg.Register("panic", func(_ context.Context, _ []byte) error {
		panic("something went wrong")
	})

	dl := &mockDeadLetter{}
	b := &retry.Backoff{Base: 0, Max: 0, JitterMax: 0}
	p := NewProcessor(mq, reg, b, dl)
	ctx := context.Background()

	job, _ := domain.NewJob("panic", nil, 1)
	_ = mq.Enqueue(ctx, job)

	// Should not panic.
	p.Process(ctx)

	stats, _ := mq.Stats(ctx)
	if stats.Dead != 1 {
		t.Fatalf("expected 1 dead after panic, got %d", stats.Dead)
	}
}

func TestProcessor_UnknownType(t *testing.T) {
	t.Parallel()

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	dl := &mockDeadLetter{}
	b := &retry.Backoff{Base: 0, Max: 0, JitterMax: 0}
	p := NewProcessor(mq, reg, b, dl)
	ctx := context.Background()

	job, _ := domain.NewJob("unknown", nil, 1)
	_ = mq.Enqueue(ctx, job)

	p.Process(ctx)

	stats, _ := mq.Stats(ctx)
	if stats.Dead != 1 {
		t.Fatalf("expected 1 dead for unknown type, got %d", stats.Dead)
	}
}
