# fxrates-service

Asynchronous FX rates microservice in Go 1.22 (Clean Architecture).

## Configuration

- Supported currency pairs are limited to combinations of `USD`, `EUR`, and `MXN`. Requests outside this set return `ErrUnsupportedPair`.
- The `exchangeratesapi` provider computes cross rates from the API's EUR base so it works even on plans that disallow changing the base.
- Configure provider credentials via `PROVIDER=exchangeratesapi`, `EXCHANGE_API_BASE=https://api.exchangeratesapi.io`, and `EXCHANGE_API_KEY=<your-key>`.

## Run

```bash
go run ./cmd/api
curl :8080/healthz

Test
go test ./... -race -count=1

Docker
docker build -t fxrates:dev .
docker run --rm -p 8080:8080 fxrates:dev
```

## gRPC worker mode and client-side load balancing

When `WORKER_TYPE=grpc`, the API returns 202 immediately and performs the fetch via gRPC in the background, persisting the result to Postgres.

- Worker listens on `GRPC_ADDR` (e.g. `:9090`).
- API dials `GRPC_TARGET` and uses gRPC `round_robin` policy.
- Set `GRPC_TARGET` to a DNS target with `dns:///` scheme, e.g. `dns:///worker:9090`. Docker Compose’s internal DNS returns multiple A records for a scaled `worker` service, and gRPC will distribute requests across replicas.
- Default `GRPC_TARGET` is `dns:///worker:9090` (suitable for compose). If you omit `dns:///`, the loader will prepend it automatically.

## Dependency Injection (Wire)

This project uses Google Wire for compile-time DI of infrastructure:

- Regenerate after adding/removing providers:

```bash
wire ./internal/bootstrap
```

- Env switches:
  - `PROVIDER=fake|exchangeratesapi`
  - `WORKER_TYPE=db`
  - PG: `DATABASE_URL`
  - Redis: `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`, `IDEMPOTENCY_TTL_MS` (default 24h)

## Endpoints

### Running PG integration tests

These tests use Testcontainers and are opt-in:

```bash
TESTCONTAINERS=1 go test ./internal/infrastructure/pg -v
```

| Method | Path                         | Description         |
|-------|------------------------------|---------------------|
| GET   | /healthz                     | liveness probe      |
| GET   | /readyz                      | readiness probe     |
| POST  | /quotes/updates              | queue update        |
| GET   | /quotes/updates/{id}         | check status        |
| GET   | /quotes/last?pair=EUR/USD    | get last quote      |

## Quick test (curl)

Health:

```bash
curl -sS http://localhost:8080/healthz
```

Readiness:

```bash
curl -sS http://localhost:8080/readyz
```

Queue an update (EUR/USD):

```bash
curl -sS -X POST http://localhost:8080/quotes/updates \
  -H 'Content-Type: application/json' \
  -H 'X-Idempotency-Key: demo-1' \
  -d '{"pair":"EUR/USD"}'
```

Check update status (replace <id> with the value from the previous response):

```bash
curl -sS http://localhost:8080/quotes/updates/<id>
```

Get the last quote:

```bash
curl -sS 'http://localhost:8080/quotes/last?pair=EUR/USD'
```

