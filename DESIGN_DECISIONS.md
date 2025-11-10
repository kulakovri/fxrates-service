# Design Decisions

## Clean Architecture and Layer Boundaries

The project follows a strict Clean Architecture pattern:

- **Domain** holds pure business entities and value objects — no technical or infrastructural concerns.  
- **Application** defines use cases and ports (interfaces).  
- **Infrastructure** contains adapters for HTTP, PostgreSQL, Redis, logging, and background workers.  
- **cmd/** holds composition roots (API and Worker).

This separation ensures testability, composability, and clear ownership of responsibilities.

## Centralized Configuration

Runtime configuration is unified in a single package: `internal/config/config.go`.

- All environment variables (service port, storage backend, `DATABASE_URL`, Redis settings, provider type/URL/API key, worker type, polling interval, batch size, etc.) are read in one place.
- Sensible defaults are provided, and the resulting `Config` struct is injected into bootstrap.
- This removes scattered `os.Getenv` calls and makes configuration reproducible across local, CI, and container environments.
- Both API and Worker processes consume the same `Config` via bootstrap, keeping environment-driven behavior explicit and testable.

## Bootstrap Composition Roots

The composition roots (API and Worker) are kept minimal and delegate wiring to bootstrap:

- `bootstrap.InitAPI(ctx)` builds repositories, Redis idempotency store, rate provider, and returns a server object with a `Run(ctx)` method.
- `bootstrap.InitWorker(ctx)` builds repositories and constructs the configured worker implementation.
- Cleanup functions (closing DB/Redis) are aggregated and returned for a single deferred call per process.
- Environment-driven choices (e.g., fake vs external rate provider, polling cadence) are applied in bootstrap using the centralized `Config`.

## Immutable Quotes History

Introduced `quotes_history` as an append-only table that records all fetched FX quotes.  
The `quotes` table remains mutable and stores the latest quote per pair for quick lookups.  
This design allows temporal analysis, auditability, and future replayability without complicating read paths.

## Idempotency and Redis Integration

Initially, idempotency was considered at the database layer via a unique key on `quote_updates`,  
but we moved it to Redis for these reasons:

- Keeps business entities pure and free of transport-level details.  
- Avoids permanent storage of transient request metadata.  
- Enables TTL-based cleanup and faster access.  
- Scales horizontally for multiple API replicas.

Redis now acts as a shared, short-lived store for `X-Idempotency-Key` enforcement.  
The API rejects duplicate keys within the TTL window with HTTP 409 Conflict.

## Database Design

- PostgreSQL is the primary datastore.  
- Tables are pluralized for naming consistency.  
- Each table defines explicit data integrity constraints.  
- Migrations use **golang-migrate** with `.up.sql`/`.down.sql` symmetry for reversibility.  
- No generic `created_at`/`modified_at` added early — explicit timestamps (e.g. `updated_at`) per use case.

## Worker Model

The worker runs separately from the API process.  
It polls for queued updates from the database (`quote_updates`), fetches rates via a provider,  
and persists results back.  
This separation enables horizontal scaling and future replacement of transport channels (e.g., SQS).  
A `Worker` interface was introduced to allow multiple strategies (e.g., `DbWorker`).  
An in-memory worker exists only as a test helper (`*_test.go`) and is not shipped in binaries.

## Observability Philosophy

- Liveness (`/healthz`): confirms the process is running.  
- Readiness (`/readyz`): verifies database connectivity and migration status.  
- Structured JSON logs via Zap with request IDs.  
- `trace_id` propagation is deferred until inter-service tracing is introduced to avoid mixing cross-cutting concerns into domain or DB entities prematurely.

## Testing Strategy

- Unit tests colocated with code.  
- Integration tests use Testcontainers (Postgres) guarded by an env flag.  
- Redis tests use miniredis for deterministic behavior.  
- Tests follow the AAA pattern and use `testify/require` assertions.

## Guiding Principles

- Explicit over magic.  
- Prefer functional seams (e.g., `WithClock`, `WithIDGen`) over globals.  
- Keep technical metadata (idempotency keys, trace headers) outside domain.  
- Treat Redis, Postgres, and Workers as interchangeable adapters — ports stay stable.  
- Every process logs request and correlation identifiers for debuggability.