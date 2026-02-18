package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Job represents a unit of work in the queue.
type Job struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Payload     []byte    `json:"payload"`
	Status      Status    `json:"status"`
	Priority    int       `json:"priority"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ScheduledAt time.Time `json:"scheduled_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// NewJob creates a new job with defaults.
func NewJob(jobType string, payload []byte, maxAttempts int) (*Job, error) {
	if jobType == "" {
		return nil, fmt.Errorf("job type cannot be empty")
	}
	if maxAttempts < 1 {
		return nil, fmt.Errorf("max attempts must be at least 1")
	}

	now := time.Now()
	return &Job{
		ID:          generateID(),
		Type:        jobType,
		Payload:     payload,
		Status:      StatusPending,
		Priority:    0,
		Attempts:    0,
		MaxAttempts: maxAttempts,
		CreatedAt:   now,
		ScheduledAt: now,
	}, nil
}

// TransitionTo attempts to change the job status.
func (j *Job) TransitionTo(s Status) error {
	if err := ValidateTransition(j.Status, s); err != nil {
		return err
	}
	j.Status = s
	if s == StatusCompleted {
		j.CompletedAt = time.Now()
	}
	return nil
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
