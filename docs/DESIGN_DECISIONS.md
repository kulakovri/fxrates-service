# Design Decisions

## Clean Architecture and Layer Boundaries

The service follows a Clean Architecture layout to enforce strict separation of concerns:

- **Domain** — pure business rules, value objects, and invariants. No external dependencies.
- **Application** — use‑case orchestration plus ports/interfaces for persistence, providers, and workers.
- **Infrastructure** — adapters for PostgreSQL, Redis, HTTP client, logging, and background worker runtime.
- **cmd/** — composition roots for API and Worker processes.

This structure ensures testability, easy substitution of adapters, and predictable ownership of responsibilities.

## Centralized Configuration

All runtime configuration is provided by `internal/config/config.go`. It loads and validates environment variables for:

- **API port**
- **Worker type** (db/chan/grpc)
- **Polling intervals and batch sizes**
- **PostgreSQL URL**
- **Redis URL/DB/password and idempotency TTL**
- **FX provider base URL and API key**
- **HTTP backoff settings (initial/max/total)**
- **Log level**

Storing all configuration in one place eliminates scattered `os.Getenv` calls and makes the service reproducible across local, CI, and container environments.

## Bootstrap as Composition Root

Bootstrapping logic lives in `internal/bootstrap/` and handles:

- wiring repositories
- wiring rate provider
- wiring Redis idempotency store
- building either API or Worker process
- managing cleanup lifecycles (closing DB, Redis)

Each process has a single entry point (`InitAPI`, `InitWorker`) which returns a `(service, cleanup)` pair, keeping `cmd/api/main.go` and `cmd/worker/main.go` intentionally minimal.

## Data Model and Persistence

### Immutable History

- The `quotes_history` table records every retrieved FX rate (append‑only).
- The `quotes` table holds only the latest quote per pair.

This enables temporal analysis without complicating frequent read paths.

### Database Conventions

- Tables are pluralized.
- All constraints (PK, FK, unique keys) are explicit.
- Timestamps are added only where meaningful (e.g., `updated_at` for quotes; `completed_at` for update jobs).
- Migrations use golang‑migrate and maintain `.up.sql`/`.down.sql` symmetry.

## Idempotency With Redis

The API exposes `X-Idempotency-Key` for `POST /quotes/updates`.

Idempotency storage is implemented in Redis rather than the database because:

- idempotency is transport‑level, not a domain invariant
- Redis enables TTL‑based cleanup
- avoids polluting tables with ephemeral request metadata
- fast and horizontally scalable

A repeated key results in HTTP 409 Conflict.

## Background Worker Model

The service supports three worker modes (chan, db, grpc) that all share the same business behavior but differ in how work is transported and executed.

### What is common across all worker types

1. Job lifecycle & statuses
   - Jobs live in `quote_updates` and move through: queued → processing → done/failed.
   - A job always corresponds to one quote update for a specific FX pair.

2. Application workflow
   - For every job, the service:
     1. Marks it as processing.
     2. Calls the `RateProvider` (`Get(ctx, pair)`).
     3. Writes the result into `quotes_history` and updates `quotes` with the latest price.
     4. Marks the job as done or failed and persists the error message if any.

3. Idempotency & error handling
   - The HTTP API enforces `X-Idempotency-Key`, regardless of worker type.
   - Provider failures are surfaced as job errors and logged; the job is not retried endlessly inside the same loop.

4. Ports & interfaces
   - The application layer only depends on ports: `UpdateJobRepo`, `QuoteRepo`, `RateProvider`, and `Worker`.
   - Each mode is just a different adapter for that `Worker` abstraction.

### How the three worker modes differ

#### 1. chan mode — in‑process worker (local dev)

- Execution model: the worker runs inside the API process.
- Transport: a Go channel connects HTTP handler → worker. Jobs are not persisted before processing.
- Persistence: results are still written to Postgres, but the queue itself is in‑memory.
- Use case: local development / fast iteration; fewer moving pieces.

Key trade‑offs:

- No durability for queued jobs (they disappear if the process dies).
- Simplest to reason about; great for tests and demos.

#### 2. db mode — database‑backed polling worker

- Execution model: worker runs as a separate process/container.
- Transport: `quote_updates` acts as a durable queue.
- The worker uses `ClaimQueued` with FOR UPDATE SKIP LOCKED semantics to claim work.
- Persistence: jobs are durable; a restart does not lose queued work.
- Use case: production‑like mode with durability, backpressure, and horizontal scaling (multiple workers polling the same DB).

Key trade‑offs:

- Simpler than introducing a message broker, at the cost of using the DB as a queue.

#### 3. grpc mode — gRPC worker pool

- Execution model: API process enqueues work and pushes it over gRPC to a separate worker service. Workers run in their own containers and expose a gRPC server.
- Transport: gRPC stream/request instead of DB polling.
- Persistence: results are still written into Postgres, but the transport between API and worker is decoupled from the DB.
- Use case: models a future evolution where workers live in a separate service (or language/runtime) accessed via RPC rather than DB polling.

Key trade‑offs:

- More moving parts than db mode, but closer to a proper microservice topology.
- Good for exploring how the same application core would behave behind an RPC boundary.

## HTTP Provider Abstraction

Providers implement the application port:

```go
type RateProvider interface {
    Get(ctx context.Context, pair string) (domain.Quote, error)
}
```

Two implementations exist:

- `Fake` (for tests)
- `ExchangeRatesAPIProvider` (real external API)

The HTTP client wrapper (`httpx.Client`) includes JSON decoding and retry with exponential backoff; non‑200 responses are surfaced cleanly so workers can record failures.

## Structured Logging

Logging uses Zap in production mode with:

- JSON output
- ISO8601 timestamps
- log level configurable via environment

At present the base logger is used everywhere; contextual fields (e.g., `request_id`, `trace_id`, service mode) can be added via `logx.WithFields(ctx)` when those values are propagated. There is no metrics layer or distributed tracing in scope.

This keeps observability minimal, vendor‑agnostic, and test‑friendly.

## Testing Strategy

- Unit tests colocated with code
- DB integration tests using Testcontainers for Postgres (optional)
- Redis tests using miniredis for deterministic behavior
- Tests follow AAA and prefer `testify/require`

The design avoids global state and prefers small, injectable dependencies.

## Guiding Architectural Principles

- Explicit over magic — no hidden global singletons beyond the logger.
- Dependency inversion — domain does not import infrastructure.
- No cross‑layer leakage — HTTP headers, Redis keys, and tracing IDs stay out of domain entities.
- Replaceability — databases, providers, and worker strategies can change without rewriting business logic.
- Keep the system boring — avoid unnecessary complexity (no metrics stack, no distributed tracing) for a small service.