package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devaloi/workq/internal/config"
	"github.com/devaloi/workq/internal/deadletter"
	"github.com/devaloi/workq/internal/domain"
	"github.com/devaloi/workq/internal/handler"
	"github.com/devaloi/workq/internal/queue"
	"github.com/devaloi/workq/internal/retry"
	"github.com/devaloi/workq/internal/worker"
)

func main() {
	cfg := config.FromEnv()

	// Create queue (persistent if path configured).
	var q queue.Queue
	mq := queue.NewMemoryQueue()
	if cfg.PersistPath != "" {
		pq, err := queue.NewPersistentQueue(cfg.PersistPath)
		if err != nil {
			log.Fatalf("Failed to create persistent queue: %v", err)
		}
		q = pq
		defer pq.Close()
	} else {
		q = mq
		defer mq.Close()
	}

	// Set up handler registry.
	reg := handler.NewRegistry()
	registerHandlers(reg)

	// Set up backoff strategy.
	backoff := &retry.Backoff{
		Base:      cfg.BackoffBase,
		Max:       cfg.BackoffMax,
		JitterMax: cfg.JitterMax,
	}

	// Set up dead letter store.
	dl := deadletter.NewStore()

	// Create processor and pool.
	proc := worker.NewProcessor(q, reg, backoff, dl)
	pool := worker.NewPool(proc, cfg.Concurrency)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Enqueue demo jobs.
	enqueueDemoJobs(ctx, q)

	// Start workers.
	pool.Start(ctx)

	// Wait for signal or completion.
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	// Wait for jobs to complete or signal.
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			stats, _ := q.Stats(ctx)
			if stats.Pending == 0 && stats.Active == 0 {
				time.Sleep(500 * time.Millisecond) // grace period
				done <- syscall.SIGINT
				return
			}
		}
	}()

	<-done
	fmt.Println("\n--- Shutting down ---")
	pool.Shutdown(cfg.ShutdownTimeout)

	// Print final stats.
	stats, _ := q.Stats(context.Background())
	fmt.Println("\n=== Final Stats ===")
	fmt.Printf("  Completed: %d\n", stats.Completed)
	fmt.Printf("  Failed:    %d (total failures including retries)\n", stats.Failed)
	fmt.Printf("  Dead:      %d\n", stats.Dead)
	fmt.Printf("  Pending:   %d\n", stats.Pending)

	// Print dead letter entries.
	deadJobs := dl.List()
	if len(deadJobs) > 0 {
		fmt.Printf("\n=== Dead Letter Queue (%d) ===\n", len(deadJobs))
		for _, j := range deadJobs {
			fmt.Printf("  [%s] type=%s attempts=%d error=%q\n", j.ID[:8], j.Type, j.Attempts, j.Error)
		}
	}
}

func registerHandlers(reg *handler.Registry) {
	_ = reg.Register("email_send", func(ctx context.Context, payload []byte) error {
		var data struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		// Simulate work with ~30% failure rate.
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		if rand.Float64() < 0.3 {
			return fmt.Errorf("SMTP timeout sending to %s", data.To)
		}

		log.Printf("✓ Email sent to %s: %s", data.To, data.Subject)
		return nil
	})

	_ = reg.Register("image_resize", func(ctx context.Context, payload []byte) error {
		var data struct {
			ImageID string `json:"image_id"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		// Simulate work with ~20% failure rate.
		time.Sleep(time.Duration(100+rand.Intn(200)) * time.Millisecond)
		if rand.Float64() < 0.2 {
			return fmt.Errorf("out of memory resizing %s", data.ImageID)
		}

		log.Printf("✓ Resized image %s to %dx%d", data.ImageID, data.Width, data.Height)
		return nil
	})
}

func enqueueDemoJobs(ctx context.Context, q queue.Queue) {
	recipients := []string{"alice@example.com", "bob@example.com", "carol@example.com", "dave@example.com", "eve@example.com"}

	for i := 0; i < 10; i++ {
		payload, _ := json.Marshal(map[string]string{
			"to":      recipients[i%len(recipients)],
			"subject": fmt.Sprintf("Newsletter #%d", i+1),
		})
		job, _ := domain.NewJob("email_send", payload, 3)
		if i < 3 {
			job.Priority = 1 // high priority for first 3
		} else {
			job.Priority = 5
		}
		_ = q.Enqueue(ctx, job)
	}

	for i := 0; i < 10; i++ {
		payload, _ := json.Marshal(map[string]interface{}{
			"image_id": fmt.Sprintf("img_%04d", i+1),
			"width":    800,
			"height":   600,
		})
		job, _ := domain.NewJob("image_resize", payload, 3)
		job.Priority = 3
		_ = q.Enqueue(ctx, job)
	}

	log.Println("Enqueued 20 demo jobs (10 emails + 10 image resizes)")
}
