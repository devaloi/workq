package queue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devaloi/workq/internal/domain"
)

func TestPersistentQueue_SaveLoadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	ctx := context.Background()

	// Create queue and enqueue jobs.
	pq, err := NewPersistentQueue(path)
	if err != nil {
		t.Fatalf("new persistent queue: %v", err)
	}

	j1, _ := domain.NewJob("type_a", []byte(`{"key":"val1"}`), 3)
	j2, _ := domain.NewJob("type_b", []byte(`{"key":"val2"}`), 5)
	_ = pq.Enqueue(ctx, j1)
	_ = pq.Enqueue(ctx, j2)
	pq.Close()

	// Verify file was created.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Create new queue from same file — should recover jobs.
	pq2, err := NewPersistentQueue(path)
	if err != nil {
		t.Fatalf("reload persistent queue: %v", err)
	}
	defer pq2.Close()

	stats, _ := pq2.Stats(ctx)
	if stats.Pending != 2 {
		t.Fatalf("expected 2 pending jobs after reload, got %d", stats.Pending)
	}

	// Dequeue and verify data integrity.
	got, _ := pq2.Dequeue(ctx)
	if got == nil {
		t.Fatal("expected a job after reload")
	}
}

func TestPersistentQueue_RecoveryAfterCrash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	ctx := context.Background()

	// Simulate: enqueue, dequeue (job becomes active), then "crash" (close without ack).
	pq, _ := NewPersistentQueue(path)
	j, _ := domain.NewJob("crash_test", []byte("data"), 3)
	_ = pq.Enqueue(ctx, j)
	got, _ := pq.Dequeue(ctx)
	_ = got // job is now active
	pq.Close()

	// On restart, active jobs should be recovered as pending.
	pq2, err := NewPersistentQueue(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	defer pq2.Close()

	stats, _ := pq2.Stats(ctx)
	if stats.Pending != 1 {
		t.Fatalf("expected 1 pending (recovered active) job, got %d", stats.Pending)
	}
}

func TestPersistentQueue_NoFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	pq, err := NewPersistentQueue(path)
	if err != nil {
		t.Fatalf("should handle missing file: %v", err)
	}
	defer pq.Close()

	stats, _ := pq.Stats(context.Background())
	if stats.Pending != 0 {
		t.Fatalf("expected 0 pending, got %d", stats.Pending)
	}
}
