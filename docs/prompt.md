# Build workq — Background Job Queue in Go

You are building a **portfolio project** for a Senior AI Engineer's public GitHub. It must be impressive, clean, and production-grade. Read these docs before writing any code:

1. **`G04-go-job-queue.md`** — Complete project spec: architecture, phases, Redis Lua scripts, worker pool design, commit plan. This is your primary blueprint. Follow it phase by phase.
2. **`github-portfolio.md`** — Portfolio goals and Definition of Done (Level 1 + Level 2). Understand the quality bar.
3. **`github-portfolio-checklist.md`** — Pre-publish checklist. Every item must pass before you're done.

---

## Instructions

### Read first, build second
Read all three docs completely before writing a single line of code. Understand the Redis sorted set queue, the Lua atomic operations, the worker pool with context cancellation, and the retry backoff strategy.

### Follow the phases in order
The project spec has 4 phases. Do them in order:
1. **Foundation** — project setup, core types (Job, JobState, JobOptions), config, Redis connection
2. **Queue** — enqueue/dequeue with Redis sorted sets, Lua scripts for atomic operations, priority support, scheduled jobs
3. **Worker Pool** — configurable worker count, per-worker goroutines, context cancellation, graceful shutdown with drain
4. **Retry + Polish** — exponential backoff, max retries, dead letter queue, comprehensive tests with miniredis, refactor, README

### Commit frequently
Follow the commit plan in the spec. Use **conventional commits**. Each commit should be a logical unit.

### Quality non-negotiables
- **Redis Lua scripts.** Atomic dequeue must use Lua (EVAL) — not RPOPLPUSH or other multi-command sequences. This shows Redis beyond basic commands.
- **miniredis for tests.** Use `alicebob/miniredis` for an in-memory Redis. Tests must not require a running Redis server. Fast, deterministic, CI-friendly.
- **Worker pool with graceful shutdown.** Workers receive `context.Context`. On shutdown signal, stop accepting new jobs, wait for in-flight jobs to complete (with timeout), then exit. No goroutine leaks.
- **Exponential backoff.** Failed jobs retry with increasing delays (e.g., 1s, 5s, 25s, 2min). Configurable base and max delay. Jitter to prevent thundering herd.
- **Job lifecycle tracking.** Jobs have states: pending → active → completed/failed/dead. Timestamps at each transition. Queryable.
- **Priority queues.** Jobs can have priority. Higher priority dequeued first. Implemented via Redis sorted set scores.
- **Lint clean.** `golangci-lint run` and `go vet` must pass.
- **No Docker.** Just `go build` and `go run`. Tests use miniredis.

### What NOT to do
- Don't use Asynq, Machinery, or any job queue library. Build from Redis primitives.
- Don't skip Lua scripts. Multi-command sequences are not atomic — use EVAL.
- Don't require a running Redis for tests. Use miniredis.
- Don't let workers leak goroutines. Every goroutine must respond to context cancellation.
- Don't leave `// TODO` or `// FIXME` comments anywhere.
- Don't commit any Redis configuration files or data.

---

## GitHub Username

The GitHub username is **devaloi**. For Go module paths, use `github.com/devaloi/workq`. All internal imports must use this module path.

## Start

Read the three docs. Then begin Phase 1 from `G04-go-job-queue.md`.
