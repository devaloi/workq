# workq

A background job queue in Go with in-memory priority scheduling, worker pools, exponential backoff retries, dead letter handling, and graceful shutdown.

**Zero external dependencies.** Clone and run immediately — no Redis, no Docker, no setup.

```
go run ./cmd/workq/
```

---

## Features

- **Priority queue** — heap-based scheduling, lower number = higher priority
- **Worker pool** — configurable concurrency with per-worker goroutines
- **Retry with backoff** — exponential backoff + jitter, configurable base/max
- **Dead letter queue** — inspect, retry, or purge permanently failed jobs
- **Graceful shutdown** — drain in-flight jobs on SIGTERM with configurable timeout
- **Optional persistence** — JSON file snapshots for crash recovery
- **Delayed scheduling** — jobs with future `ScheduledAt` aren't dequeued early
- **Panic recovery** — handler panics are caught and treated as failures

---

## Job Lifecycle

```
                    ┌─────────────────────────────┐
                    │         Enqueue              │
                    └──────────┬──────────────────┘
                               ▼
                         ┌──────────┐
                         │ PENDING  │◄──────────────┐
                         └────┬─────┘               │
                              │ Dequeue             │ Retry
                              ▼                     │ (attempts < max)
                         ┌──────────┐               │
                         │  ACTIVE  │───────────────┘
                         └────┬─────┘
                              │
                    ┌─────────┴──────────┐
                    │                    │
                    ▼                    ▼
             ┌────────────┐       ┌──────────┐
             │ COMPLETED  │       │   DEAD   │
             └────────────┘       └──────────┘
                                  (attempts >= max)
```

---

## Quick Start

```bash
# Clone
git clone https://github.com/devaloi/workq.git
cd workq

# Run demo (20 jobs, 4 workers, mixed types)
go run ./cmd/workq/

# Run tests
go test -race ./...

# Build binary
make build
```

---

## Adding a Handler

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/devaloi/workq/internal/handler"
)

func main() {
    reg := handler.NewRegistry()

    reg.Register("email_send", func(ctx context.Context, payload []byte) error {
        var data struct {
            To      string `json:"to"`
            Subject string `json:"subject"`
        }
        if err := json.Unmarshal(payload, &data); err != nil {
            return err
        }
        fmt.Printf("Sending email to %s: %s\n", data.To, data.Subject)
        return nil
    })
}
```

---

## Enqueuing Jobs

```go
job, _ := domain.NewJob("email_send", payload, 3) // type, payload, maxAttempts
job.Priority = 1  // lower = higher priority (default: 0)
queue.Enqueue(ctx, job)
```

---

## Configuration

All settings via environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKQ_CONCURRENCY` | `4` | Number of worker goroutines |
| `WORKQ_MAX_RETRIES` | `5` | Max attempts before dead letter |
| `WORKQ_BACKOFF_BASE` | `1s` | Base delay for exponential backoff |
| `WORKQ_BACKOFF_MAX` | `5m` | Maximum backoff delay |
| `WORKQ_JITTER_MAX` | `500ms` | Max jitter added to backoff |
| `WORKQ_PERSIST_PATH` | _(empty)_ | File path for JSON persistence |
| `WORKQ_SHUTDOWN_TIMEOUT` | `30s` | Max wait for in-flight jobs |

---

## Architecture

```
workq/
├── cmd/workq/           # Demo binary
├── internal/
│   ├── config/          # Environment-based configuration
│   ├── domain/          # Job struct, status enum, transitions
│   ├── queue/           # Queue interface, memory + persistent implementations
│   ├── worker/          # Worker pool and job processor
│   ├── handler/         # Handler registry (job type → function)
│   ├── retry/           # Exponential backoff with jitter
│   ├── deadletter/      # Dead letter store (list, retry, purge)
│   └── integration/     # End-to-end tests
├── examples/
│   ├── email/           # Email job with random failures
│   └── pipeline/        # Multi-stage pipeline (fetch→process→store)
```

---

## Examples

```bash
# Email sending with retries
go run ./examples/email/

# Multi-stage pipeline
go run ./examples/pipeline/
```

---

## Testing

```bash
# All tests with race detector
go test -race -count=1 ./...

# Verbose output
go test -race -v ./...

# Coverage report
make coverage
```

---

## License

MIT — see [LICENSE](LICENSE)
