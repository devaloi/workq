# Changelog

All notable changes to workq are documented here.

## [0.2.0] - 2026-02-20

### Added
- Job deduplication by key
- Rate limiting per job type
- GitHub Actions CI pipeline

## [0.1.0] - 2026-02-18

### Added
- In-memory priority queue with configurable worker pool
- Exponential backoff retry with jitter
- Dead letter queue for failed jobs
- Graceful shutdown: drains in-flight jobs
- Metrics: throughput, queue depth, failure rate
- MIT License
