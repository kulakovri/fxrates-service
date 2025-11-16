# fxrates-service

## TL;DR — Quick Start

Build and run all three worker modes locally (chan, db, grpc) using docker-compose.

```bash
./run.sh -b
```

This spins up:

| Mode | URL |
|---|---|
| chan | http://localhost:8081 |
| db | http://localhost:8082 |
| gRPC | http://localhost:8083 |

Each instance has its own Postgres + Redis, so queues and data do not interfere.

Swagger UI is available at:

```
http://localhost:<port>/swagger
```

Asynchronous FX rates microservice written in Go 1.23.

Implements three worker strategies (chan, db, grpc) and exposes a uniform HTTP API with idempotency, persistence, and background processing.

## Documentation

- [Architecture Overview](docs/ARCHITECTURE.md)
- [Design Decisions](docs/DESIGN_DECISIONS.md)

## Quick Start (Local)

The project can run three isolated service stacks in parallel, each representing a different worker mode:

| Mode | Local URL | Description |
|---|---|---|
| chan | http://localhost:8081 | In-process channel worker |
| db | http://localhost:8082 | Dedicated DB-backed background worker |
| grpc | http://localhost:8083 | API dispatches to gRPC worker pool |

Each stack runs its own Postgres and Redis, so environments never interfere with each other.

Every stack exposes Swagger UI at `/swagger/`.

Examples:

- http://localhost:8081/swagger/
- http://localhost:8082/swagger/
- http://localhost:8083/swagger/

## Running Locally With Docker Compose

1) Build the image:

```bash
docker build -t fxrates:dev .
```

2) Run any mode:

Chan (in-process worker):

```bash
docker compose -f ops/docker/docker-compose.yml -p fxrates-chan --profile chan up -d
```

DB (background DB-polling worker):

```bash
docker compose -f ops/docker/docker-compose.yml -p fxrates-db --profile db up -d
```

gRPC (API + gRPC workers):

```bash
docker compose -f ops/docker/docker-compose.yml -p fxrates-grpc --profile grpc up -d
```

Note: In local runs, the service automatically uses the fake FX provider (no external API calls).

## Running with Ansible (Local or EC2)

In addition to plain Docker Compose, the project supports Ansible-driven deployments. This is used both for local runs with real API keys and for CI → EC2 deploy.

1) Create `.vault-pass.txt`

Ansible decrypts secrets via a vault password. Reviewers must create a file in the project root:

```bash
echo "<vault password>" > .vault-pass.txt
```

Hint for reviewers: the password is 5 lowercase letters, equal to the company name used throughout the project. (Not included here for security reasons.) Keep this file local only — never commit it.

2) Run any worker mode using Ansible

Instead of plain Compose, use:

```bash
ansible-playbook ops/compose.dev.yml --vault-password-file .vault-pass.txt
```

What Ansible does:

- Provides real ExchangeRatesAPI credentials via decrypted vault
- Exports env variables for the service
- Calls the corresponding Docker Compose profile(s)
- Ensures reproducible configuration between local and EC2 environments

## Remote Deployment (EC2)

A GitHub Actions workflow + Ansible playbook deploys the service to an EC2 instance.

Remote base URLs:

```json
{
  "chan": { "baseUrl": "http://localhost:8081" },
  "db": { "baseUrl": "http://localhost:8082" },
  "gRPC": { "baseUrl": "http://localhost:8083" },
  "chan-ec2": { "baseUrl": "http://3.148.167.60/chan" },
  "db-ec2": { "baseUrl": "http://3.148.167.60/db" },
  "gRPC-ec2": { "baseUrl": "http://3.148.167.60/grpc" }
}
```

EC2 Notes:

- The Ansible playbook injects real ExchangeRatesAPI keys (used only in deployed mode).
- Local Docker Compose does not use these keys.
- To run the deploy manually you need:
  - ansible
  - Docker Compose plugin (docker compose)

## Configuration

All configuration is centralized in `internal/config`.

Important environment variables:

| Variable | Description |
|---|---|
| WORKER_TYPE | chan, db, or grpc |
| PROVIDER | fake (default) or exchangeratesapi |
| EXCHANGE_API_BASE | API base URL |
| EXCHANGE_API_KEY | Provider key (only needed in deployed mode) |
| DATABASE_URL | Connection string |
| REDIS_ADDR | Redis instance |
| IDEMPOTENCY_TTL_MS | Default: 24h |

Supported currency pairs: combinations of USD, EUR, MXN.

## Worker Modes (Conceptual Summary)

| Mode | Where Worker Runs | Queue | Durability | Notes |
|---|---|---|---|---|
| chan | Inside the API process | Go channel | None | Fastest; restarts lose jobs |
| db | Separate worker container | PostgreSQL (quote_updates) | Durable | Good balance for production |
| grpc | Dedicated gRPC worker pool | RPC calls | Depends on retries | Best for multi-service topologies |

### Operational Characteristics (compact)

| Aspect | chan | db | grpc |
|---|---|---|---|
| Horizontal scaling | API-only | API + workers | API + workers |
| Job loss on restart | Yes | No | Depends on caller |
| Back-pressure | Channel buffer | DB queue | RPC timeouts |
| Complexity | Low | Medium | Highest |
| Typical use | Local/dev | Prod-like | Distributed systems |

## gRPC Worker Mode

- API sends background jobs to a worker pool using `GRPC_TARGET`.
- Built-in load balancing via `round_robin`.
- Compose DNS auto-discovers multiple workers:

```bash
GRPC_TARGET=dns:///worker:9090
```

## Integration & E2E Tests

Postgres tests:

```bash
TESTCONTAINERS=1 go test ./internal/infrastructure/pg -v
```

E2E tests across all worker modes:

```bash
docker build -t fxrates:dev .
E2E_PROFILES=1 go test -tags=e2e ./internal/integration -v
```

## Dependency Injection (Google Wire)

Regenerate when provider constructors change:

```bash
wire ./internal/bootstrap
```

## Endpoints

All modes expose the same API:

| Method | Path | Description |
|---|---|---|
| GET | /healthz | Liveness |
| GET | /readyz | Readiness |
| POST | /quotes/updates | Queue a quote update |
| GET | /quotes/updates/{id} | Check update status |
| GET | /quotes/last?pair=EUR/USD | Fetch last quote |

### Quick curl test

```bash
curl -sS -X POST http://localhost:8081/quotes/updates \
  -H 'Content-Type: application/json' \
  -H 'X-Idempotency-Key: demo-1' \
  -d '{"pair":"EUR/USD"}'
```

## Summary for Reviewers

- Clone → build → run any of the 3 modes in seconds.
- Swagger available for each running stack.
- Idempotency built into the API (Redis).
- DB-backed queue, gRPC worker pool, and in-process worker all available.
- Local mode uses a fake FX provider (no external calls).
- Deployment to EC2 automated via GitHub Actions + Ansible.


