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
	log.Println("=== Multi-Stage Pipeline Example ===")

	mq := queue.NewMemoryQueue()
	defer mq.Close()

	reg := handler.NewRegistry()

	// Stage 1: Fetch data.
	_ = reg.Register("fetch", func(ctx context.Context, payload []byte) error {
		var data struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return fmt.Errorf("unmarshaling fetch payload: %w", err)
		}
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		log.Printf("  📥 Fetched: %s", data.URL)

		// Enqueue next stage.
		next, _ := json.Marshal(map[string]string{"url": data.URL, "status": "fetched"})
		job, _ := domain.NewJob("process", next, 3)
		job.Priority = 2
		return mq.Enqueue(ctx, job)
	})

	// Stage 2: Process data.
	_ = reg.Register("process", func(ctx context.Context, payload []byte) error {
		var data struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return fmt.Errorf("unmarshaling process payload: %w", err)
		}
		time.Sleep(time.Duration(100+rand.Intn(150)) * time.Millisecond)

		if rand.Float64() < 0.2 {
			return fmt.Errorf("processing failed for %s", data.URL)
		}

		log.Printf("  ⚙ Processed: %s", data.URL)

		// Enqueue final stage.
		next, _ := json.Marshal(map[string]string{"url": data.URL, "status": "processed"})
		job, _ := domain.NewJob("store", next, 3)
		job.Priority = 3
		return mq.Enqueue(ctx, job)
	})

	// Stage 3: Store results.
	_ = reg.Register("store", func(ctx context.Context, payload []byte) error {
		var data struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return fmt.Errorf("unmarshaling store payload: %w", err)
		}
		time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)
		log.Printf("  💾 Stored: %s", data.URL)
		return nil
	})

	backoff := &retry.Backoff{Base: 10 * time.Millisecond, Max: 100 * time.Millisecond, JitterMax: 5 * time.Millisecond}
	dl := deadletter.NewStore()
	proc := worker.NewProcessor(mq, reg, backoff, dl)
	pool := worker.NewPool(proc, 4)

	ctx := context.Background()

	urls := []string{
		"https://api.example.com/data/1",
		"https://api.example.com/data/2",
		"https://api.example.com/data/3",
		"https://api.example.com/data/4",
		"https://api.example.com/data/5",
	}

	for _, url := range urls {
		payload, _ := json.Marshal(map[string]string{"url": url})
		job, _ := domain.NewJob("fetch", payload, 3)
		job.Priority = 1
		_ = mq.Enqueue(ctx, job)
	}
	log.Printf("Enqueued %d pipeline jobs (fetch → process → store)", len(urls))

	pool.Start(ctx)
	time.Sleep(5 * time.Second)
	pool.Shutdown(5 * time.Second)

	stats, _ := mq.Stats(ctx)
	fmt.Printf("\nCompleted: %d | Failed: %d | Dead: %d\n", stats.Completed, stats.Failed, stats.Dead)

	if dead := dl.List(); len(dead) > 0 {
		fmt.Printf("\nDead letters (%d):\n", len(dead))
		for _, j := range dead {
			fmt.Printf("  [%s] %s → %s\n", j.ID[:8], j.Type, j.Error)
		}
	}
}
