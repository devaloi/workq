package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devaloi/workq/internal/domain"
	"github.com/devaloi/workq/internal/handler"
	"github.com/devaloi/workq/internal/queue"
	"github.com/devaloi/workq/internal/retry"
)

// DeadLetterAdder is the interface for adding jobs to the dead letter store.
type DeadLetterAdder interface {
	Add(job *domain.Job)
}

// Processor handles execution of a single job.
type Processor struct {
	queue    queue.Queue
	registry *handler.Registry
	backoff  *retry.Backoff
	dead     DeadLetterAdder
}

// NewProcessor creates a new job processor.
func NewProcessor(q queue.Queue, r *handler.Registry, b *retry.Backoff, d DeadLetterAdder) *Processor {
	return &Processor{
		queue:    q,
		registry: r,
		backoff:  b,
		dead:     d,
	}
}

// Process dequeues and executes a single job. Returns false if context is done.
func (p *Processor) Process(ctx context.Context) bool {
	job, err := p.queue.Dequeue(ctx)
	if err != nil {
		return false
	}

	execErr := p.execute(ctx, job)

	if execErr == nil {
		if ackErr := p.queue.Ack(ctx, job.ID); ackErr != nil {
			log.Printf("workq: ack error job=%s: %v", job.ID, ackErr)
		}
		return true
	}

	// Job failed — set backoff delay for retry.
	if p.backoff != nil {
		delay := p.backoff.NextDelay(job.Attempts)
		job.ScheduledAt = time.Now().Add(delay)
	}

	if failErr := p.queue.Fail(ctx, job.ID, execErr); failErr != nil {
		log.Printf("workq: fail error job=%s: %v", job.ID, failErr)
	}

	// Check if the job has exceeded max attempts (Fail increments Attempts).
	// The queue.Fail marks it dead internally; add to dead letter store.
	if job.Attempts >= job.MaxAttempts && p.dead != nil {
		job.Status = domain.StatusDead
		p.dead.Add(job)
	}

	return true
}

// execute runs the handler with panic recovery.
func (p *Processor) execute(ctx context.Context, job *domain.Job) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic: %v", r)
		}
	}()

	h, err := p.registry.Lookup(job.Type)
	if err != nil {
		return fmt.Errorf("handler lookup for %s: %w", job.Type, err)
	}

	return h(ctx, job.Payload)
}
