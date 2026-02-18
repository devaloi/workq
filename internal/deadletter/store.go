package deadletter

import (
	"fmt"
	"sync"

	"github.com/devaloi/workq/internal/domain"
)

// Store holds jobs that have exceeded their max retry attempts.
type Store struct {
	mu   sync.Mutex
	jobs map[string]*domain.Job
}

// NewStore creates an empty dead letter store.
func NewStore() *Store {
	return &Store{
		jobs: make(map[string]*domain.Job),
	}
}

// Add puts a job into the dead letter store.
func (s *Store) Add(job *domain.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *job
	cp.Status = domain.StatusDead
	s.jobs[cp.ID] = &cp
}

// List returns all dead letter jobs.
func (s *Store) List() []*domain.Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*domain.Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		cp := *j
		result = append(result, &cp)
	}
	return result
}

// Get returns a specific dead letter job.
func (s *Store) Get(id string) (*domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %s not found in dead letter store", id)
	}
	cp := *j
	return &cp, nil
}

// Retry removes a job from the dead letter store and returns it
// with status reset to pending for re-enqueue.
func (s *Store) Retry(id string) (*domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %s not found in dead letter store", id)
	}

	delete(s.jobs, id)
	j.Status = domain.StatusPending
	j.Attempts = 0
	j.Error = ""
	return j, nil
}

// Purge removes all jobs from the dead letter store.
func (s *Store) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.jobs)
	s.jobs = make(map[string]*domain.Job)
	return count
}

// Len returns the number of dead letter jobs.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs)
}
