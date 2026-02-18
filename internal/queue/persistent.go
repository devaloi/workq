package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/devaloi/workq/internal/domain"
)

// PersistentQueue wraps MemoryQueue and snapshots state to a JSON file.
type PersistentQueue struct {
	mq   *MemoryQueue
	path string
	mu   sync.Mutex
}

// NewPersistentQueue creates a queue that persists state to the given file path.
func NewPersistentQueue(path string) (*PersistentQueue, error) {
	mq := NewMemoryQueue()
	pq := &PersistentQueue{
		mq:   mq,
		path: path,
	}

	// Wire up change notification for auto-snapshot.
	mq.OnChange = func(jobs []*domain.Job) {
		pq.snapshotWith(jobs)
	}

	// Load existing state if file exists.
	if err := pq.load(); err != nil {
		return nil, fmt.Errorf("loading persistent state: %w", err)
	}

	return pq, nil
}

// persistedState is the JSON serialization format.
type persistedState struct {
	Jobs []*domain.Job `json:"jobs"`
}

func (pq *PersistentQueue) load() error {
	data, err := os.ReadFile(pq.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parsing state file: %w", err)
	}

	if len(state.Jobs) > 0 {
		pq.mq.Restore(state.Jobs)
	}
	return nil
}

func (pq *PersistentQueue) snapshotWith(jobs []*domain.Job) {
	state := persistedState{Jobs: jobs}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	// Write atomically: write to temp file, then rename.
	tmp := pq.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, pq.path)
}

func (pq *PersistentQueue) Enqueue(ctx context.Context, job *domain.Job) error {
	return pq.mq.Enqueue(ctx, job)
}

func (pq *PersistentQueue) Dequeue(ctx context.Context) (*domain.Job, error) {
	return pq.mq.Dequeue(ctx)
}

func (pq *PersistentQueue) Ack(ctx context.Context, id string) error {
	return pq.mq.Ack(ctx, id)
}

func (pq *PersistentQueue) Fail(ctx context.Context, id string, jobErr error) error {
	return pq.mq.Fail(ctx, id, jobErr)
}

func (pq *PersistentQueue) Stats(ctx context.Context) (*Stats, error) {
	return pq.mq.Stats(ctx)
}

func (pq *PersistentQueue) Close() {
	pq.mq.Close()
}
