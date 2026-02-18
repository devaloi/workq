package queue

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devaloi/workq/internal/domain"
)

// jobHeap implements heap.Interface for priority-based job scheduling.
type jobHeap []*domain.Job

func (h jobHeap) Len() int { return len(h) }
func (h jobHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].ScheduledAt.Before(h[j].ScheduledAt)
}
func (h jobHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *jobHeap) Push(x any) {
	*h = append(*h, x.(*domain.Job))
}

func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// MemoryQueue is a thread-safe in-memory priority queue.
type MemoryQueue struct {
	mu        sync.Mutex
	cond      *sync.Cond
	pending   jobHeap
	active    map[string]*domain.Job
	completed int
	failed    int
	dead      int
	closed    bool

	// OnChange is called after state mutations (for persistence layer).
	// Called with mu held — must not re-lock.
	OnChange func(jobs []*domain.Job)
}

// NewMemoryQueue creates a new in-memory queue.
func NewMemoryQueue() *MemoryQueue {
	mq := &MemoryQueue{
		active: make(map[string]*domain.Job),
	}
	mq.cond = sync.NewCond(&mq.mu)
	heap.Init(&mq.pending)
	return mq
}

func (mq *MemoryQueue) notifyChange() {
	if mq.OnChange != nil {
		mq.OnChange(mq.snapshotUnlocked())
	}
}

// Enqueue adds a job to the queue.
func (mq *MemoryQueue) Enqueue(_ context.Context, job *domain.Job) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.closed {
		return fmt.Errorf("queue is closed")
	}

	job.Status = domain.StatusPending
	heap.Push(&mq.pending, job)
	mq.cond.Signal()
	mq.notifyChange()
	return nil
}

// Dequeue blocks until a job is available or the context is cancelled.
func (mq *MemoryQueue) Dequeue(ctx context.Context) (*domain.Job, error) {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			mq.cond.Broadcast()
		case <-done:
		}
	}()

	mq.mu.Lock()
	defer mq.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if mq.closed {
			return nil, fmt.Errorf("queue is closed")
		}

		now := time.Now()
		bestIdx := -1
		var earliest time.Time

		// Scan for the best ready job (ready = ScheduledAt <= now).
		for i, j := range mq.pending {
			if j.ScheduledAt.After(now) {
				if earliest.IsZero() || j.ScheduledAt.Before(earliest) {
					earliest = j.ScheduledAt
				}
				continue
			}
			if bestIdx == -1 {
				bestIdx = i
			} else {
				cur := mq.pending[bestIdx]
				if j.Priority < cur.Priority || (j.Priority == cur.Priority && j.ScheduledAt.Before(cur.ScheduledAt)) {
					bestIdx = i
				}
			}
		}

		if bestIdx >= 0 {
			job := heap.Remove(&mq.pending, bestIdx).(*domain.Job)
			job.Status = domain.StatusActive
			mq.active[job.ID] = job
			mq.notifyChange()
			return job, nil
		}

		// No ready jobs; set a timer for the nearest delayed job.
		if !earliest.IsZero() {
			delay := time.Until(earliest)
			go func() {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-timer.C:
					mq.cond.Broadcast()
				case <-done:
				}
			}()
		}

		mq.cond.Wait()
	}
}

// Ack marks a job as completed.
func (mq *MemoryQueue) Ack(_ context.Context, id string) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if _, ok := mq.active[id]; !ok {
		return fmt.Errorf("job %s not found in active set", id)
	}
	delete(mq.active, id)
	mq.completed++
	mq.notifyChange()
	return nil
}

// Fail handles a job failure: re-enqueue if retries remain, otherwise dead letter.
func (mq *MemoryQueue) Fail(_ context.Context, id string, jobErr error) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	job, ok := mq.active[id]
	if !ok {
		return fmt.Errorf("job %s not found in active set", id)
	}
	delete(mq.active, id)

	job.Attempts++
	if jobErr != nil {
		job.Error = jobErr.Error()
	}

	if job.Attempts >= job.MaxAttempts {
		job.Status = domain.StatusDead
		mq.dead++
		mq.failed++
		mq.notifyChange()
		return nil
	}

	job.Status = domain.StatusPending
	mq.failed++
	heap.Push(&mq.pending, job)
	mq.cond.Signal()
	mq.notifyChange()
	return nil
}

// Stats returns current queue statistics.
func (mq *MemoryQueue) Stats(_ context.Context) (*Stats, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	return &Stats{
		Pending:   mq.pending.Len(),
		Active:    len(mq.active),
		Completed: mq.completed,
		Failed:    mq.failed,
		Dead:      mq.dead,
	}, nil
}

// Close shuts down the queue, unblocking all waiters.
func (mq *MemoryQueue) Close() {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	mq.closed = true
	mq.cond.Broadcast()
}

// Snapshot returns copies of all jobs for persistence.
func (mq *MemoryQueue) Snapshot() []*domain.Job {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	return mq.snapshotUnlocked()
}

// snapshotUnlocked returns copies without locking (caller must hold mu).
func (mq *MemoryQueue) snapshotUnlocked() []*domain.Job {
	jobs := make([]*domain.Job, 0, mq.pending.Len()+len(mq.active))
	for _, j := range mq.pending {
		cp := *j
		jobs = append(jobs, &cp)
	}
	for _, j := range mq.active {
		cp := *j
		cp.Status = domain.StatusPending
		jobs = append(jobs, &cp)
	}
	return jobs
}

// Restore loads jobs into the queue (used on startup).
func (mq *MemoryQueue) Restore(jobs []*domain.Job) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	for _, j := range jobs {
		j.Status = domain.StatusPending
		heap.Push(&mq.pending, j)
	}
	mq.cond.Broadcast()
}

// DeadJobs returns a copy of dead job IDs tracked by the fail path.
// Note: the full dead letter store is in the deadletter package.
func (mq *MemoryQueue) PendingJobs() []*domain.Job {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	jobs := make([]*domain.Job, len(mq.pending))
	for i, j := range mq.pending {
		cp := *j
		jobs[i] = &cp
	}
	return jobs
}
