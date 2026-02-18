package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/devaloi/workq/internal/deadletter"
	"github.com/devaloi/workq/internal/domain"
	"github.com/devaloi/workq/internal/handler"
	"github.com/devaloi/workq/internal/queue"
	"github.com/devaloi/workq/internal/retry"
	"github.com/devaloi/workq/internal/worker"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("=== Email Job Queue Example ===")

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()
	_ = reg.Register("email_send", func(ctx context.Context, payload []byte) error {
		var data struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return fmt.Errorf("unmarshaling email payload: %w", err)
		}

		time.Sleep(time.Duration(50+rand.Intn(150)) * time.Millisecond)

		// 40% failure rate to demonstrate retries.
		if rand.Float64() < 0.4 {
			return fmt.Errorf("SMTP connection timeout for %s", data.To)
		}

		log.Printf("  ✉ Sent to %s: %q", data.To, data.Subject)
		return nil
	})

	backoff := &retry.Backoff{Base: 10 * time.Millisecond, Max: 100 * time.Millisecond, JitterMax: 5 * time.Millisecond}
	dl := deadletter.NewStore()
	proc := worker.NewProcessor(mq, reg, backoff, dl)
	pool := worker.NewPool(proc, 3)

	ctx := context.Background()

	recipients := []string{"alice@co.com", "bob@co.com", "carol@co.com", "dave@co.com", "eve@co.com"}
	for i, to := range recipients {
		payload, _ := json.Marshal(map[string]string{"to": to, "subject": fmt.Sprintf("Welcome #%d", i+1)})
		job, _ := domain.NewJob("email_send", payload, 4)
		_ = mq.Enqueue(ctx, job)
	}
	log.Printf("Enqueued %d email jobs", len(recipients))

	pool.Start(ctx)
	time.Sleep(3 * time.Second)
	pool.Shutdown(5 * time.Second)

	stats, _ := mq.Stats(ctx)
	fmt.Printf("\nCompleted: %d | Failed: %d | Dead: %d\n", stats.Completed, stats.Failed, stats.Dead)

	if dead := dl.List(); len(dead) > 0 {
		fmt.Printf("\nDead letters (%d):\n", len(dead))
		for _, j := range dead {
			fmt.Printf("  %s → %s\n", j.ID[:8], j.Error)
		}
	}
}
