package worker

import (
	"context"
	"log"
	"sync"
	"time"
)

// Pool manages a group of worker goroutines.
type Pool struct {
	processor   *Processor
	concurrency int
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

// NewPool creates a worker pool with the given concurrency.
func NewPool(p *Processor, concurrency int) *Pool {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Pool{
		processor:   p,
		concurrency: concurrency,
	}
}

// Start spawns worker goroutines. Returns immediately.
func (p *Pool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)

	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go func(id int) {
			defer p.wg.Done()
			log.Printf("workq: worker %d started", id)
			for {
				if !p.processor.Process(ctx) {
					log.Printf("workq: worker %d stopped", id)
					return
				}
			}
		}(i)
	}
	log.Printf("workq: pool started with %d workers", p.concurrency)
}

// Shutdown signals workers to stop and waits up to timeout for in-flight jobs.
func (p *Pool) Shutdown(timeout time.Duration) {
	if p.cancel != nil {
		p.cancel()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("workq: pool shut down gracefully")
	case <-time.After(timeout):
		log.Println("workq: pool shutdown timed out")
	}
}
