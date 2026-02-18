# G04: workq — Background Job Queue in Go

**Catalog ID:** G04 | **Size:** S | **Language:** Go
**Repo name:** `workq`
**One-liner:** A background job queue in Go with Redis-backed persistence, worker pools, retries with backoff, and graceful shutdown.

---

## Why This Stands Out

- **Production job queue patterns** — priority queues, dead letter, retry backoff
- **Worker pool** — configurable concurrency, per-worker goroutines with context cancellation
- **Graceful shutdown** — drain in-flight jobs on SIGTERM, with configurable timeout
- **Redis Lua scripts** — atomic dequeue operations, shows Redis beyond basic GET/SET
- **Job lifecycle** — pending → active → completed/failed/dead, with timestamps at each transition
- **No framework** — not Asynq, not Machinery — built from primitives to show understanding

---

## Architecture

```
workq/
├── cmd/
│   └── workq/
│       └── main.go              # Demo: enqueue jobs, run workers
├── internal/
│   ├── config/
│   │   └── config.go            # Redis URL, concurrency, retry config
│   ├── domain/
│   │   ├── job.go               # Job struct: id, type, payload, status, attempts, timestamps
│   │   └── status.go            # Job status enum + transitions
│   ├── queue/
│   │   ├── queue.go             # Queue interface: Enqueue, Dequeue, Ack, Fail, Dead
│   │   ├── redis.go             # Redis implementation with sorted sets + Lua
│   │   ├── redis_test.go
│   │   └── scripts/             # Lua scripts for atomic operations
│   │       ├── dequeue.lua
│   │       └── retry.lua
│   ├── worker/
│   │   ├── pool.go              # Worker pool: spawn N goroutines, fan-out jobs
│   │   ├── pool_test.go
│   │   ├── processor.go         # Job processor: handler lookup, execute, retry/fail
│   │   └── processor_test.go
│   ├── handler/
│   │   ├── registry.go          # Handler registry: map job types to functions
│   │   └── registry_test.go
│   └── retry/
│       ├── backoff.go           # Exponential backoff with jitter
│       └── backoff_test.go
├── examples/
│   ├── email/main.go            # Example: email sending job
│   └── image/main.go            # Example: image resize job
├── go.mod
├── go.sum
├── Makefile
├── .env.example
├── .gitignore
├── .golangci.yml
├── LICENSE
└── README.md
```

---

## Job Lifecycle

```
Enqueue → PENDING (sorted set by priority + enqueue time)
    ↓
Dequeue → ACTIVE (moved atomically via Lua script)
    ↓
Success → COMPLETED (removed, stats incremented)
    OR
Failure (attempts < max) → PENDING (re-enqueued with backoff delay)
    OR
Failure (attempts >= max) → DEAD (moved to dead letter queue)
```

---

## Tech Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.22+ |
| Queue backend | Redis 7+ (sorted sets + Lua) |
| Testing | stdlib + miniredis (in-memory Redis for tests) |
| Linting | golangci-lint |

---

## Phased Build Plan

### Phase 1: Foundation

**1.1 — Project setup + domain types**
- `go mod init github.com/devaloi/workq`
- Job struct: ID (ULID), Type, Payload ([]byte), Status, Attempts, MaxAttempts, CreatedAt, ScheduledAt, CompletedAt
- Status: Pending, Active, Completed, Failed, Dead
- Tests: job creation, status transitions

### Phase 2: Queue

**2.1 — Redis queue**
- Queue interface: `Enqueue(job)`, `Dequeue(ctx) (*Job, error)`, `Ack(id)`, `Fail(id, err)`, `Dead(id)`
- Redis implementation using sorted sets (score = scheduled time for delayed retry)
- Lua script for atomic dequeue: ZPOPMIN from pending + ZADD to active
- Lua script for retry: move from active back to pending with new score
- Tests with miniredis: enqueue/dequeue, ordering, atomic operations

**2.2 — Dead letter queue**
- Jobs exceeding max attempts moved to dead letter sorted set
- `ListDead(limit)` — inspect dead jobs
- `RetryDead(id)` — move back to pending
- `PurgeDead()` — clear all dead jobs
- Tests: max attempts → dead, retry dead, purge

### Phase 3: Worker Pool

**3.1 — Processor**
- Handler function type: `func(ctx context.Context, payload []byte) error`
- Handler registry: map job type string → handler function
- Processor: dequeue → lookup handler → execute → ack or fail
- Tests: successful job, failed job, unknown type, panic recovery

**3.2 — Worker pool**
- Pool struct: N workers, processor, quit channel
- `Start(ctx)` — spawn N goroutines, each runs processor loop
- `Stop(timeout)` — signal quit, wait for in-flight jobs up to timeout
- Graceful shutdown: context cancellation propagated to handlers
- Tests: concurrent processing, graceful shutdown, timeout

### Phase 4: Retry + Polish

**4.1 — Exponential backoff**
- Formula: `min(base * 2^attempt + jitter, max_delay)`
- Configurable base, max, jitter range
- Tests: backoff values, jitter within range, max cap

**4.2 — Examples**
- `email/` — simulate email sending with random failures, show retry behavior
- `image/` — simulate image processing with progress

**4.3 — Integration test**
- Enqueue 100 jobs, run pool of 5 workers, verify all complete or dead
- Test graceful shutdown mid-processing

**4.4 — README**
- Badges, install, quick start
- Job lifecycle diagram
- Configuration reference
- How to add custom handlers
- Redis requirements

---

## Commit Plan

1. `chore: scaffold project with config`
2. `feat: add job domain types and status transitions`
3. `feat: add Redis queue with Lua atomic dequeue`
4. `feat: add dead letter queue`
5. `feat: add handler registry and processor`
6. `feat: add worker pool with graceful shutdown`
7. `feat: add exponential backoff with jitter`
8. `feat: add example job handlers`
9. `test: add integration tests`
10. `docs: add README with lifecycle diagram`
