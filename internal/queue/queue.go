package queue

import (
	"context"

	"github.com/devaloi/workq/internal/domain"
)

// Stats holds queue statistics.
type Stats struct {
	Pending   int `json:"pending"`
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Dead      int `json:"dead"`
}

// Queue defines the interface for job queue operations.
type Queue interface {
	Enqueue(ctx context.Context, job *domain.Job) error
	Dequeue(ctx context.Context) (*domain.Job, error)
	Ack(ctx context.Context, id string) error
	Fail(ctx context.Context, id string, jobErr error) error
	Stats(ctx context.Context) (*Stats, error)
}
