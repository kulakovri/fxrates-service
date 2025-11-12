# Architecture Overview

This service exposes an HTTP API that accepts quote update requests and processes them **in the background** using one of three modes selected by `WORKER_TYPE`: **db**, **grpc**, or **chan**. Idempotency is enforced via Redis.

## Component Diagram

```mermaid

flowchart LR

  subgraph Client

    U[HTTP Client]

  end

  subgraph API[API Process]

    A[HTTP Router\n/quotes/updates\n/quotes/updates/{id}\n/quotes/last]

    AE[Idempotency\n(X-Idempotency-Key → Redis TTL)]

    BG[Background Orchestrator]

  end

  subgraph PG[(Postgres)]

    Q[(quotes)]

    QU[(quote_updates)]

    QH[(quotes_history)]

  end

  subgraph Redis[(Redis)]

    IDEM[Idempotency keys]

  end

  subgraph WorkerDB[DbWorker (separate process)]

    WDB[Poll queue\nFOR UPDATE SKIP LOCKED]

  end

  subgraph WorkerGRPC[RateServer (gRPC)]

    WGRPC[gRPC Fetch(pair, trace_id, timeout)]

  end

  subgraph Provider[External FX API]

    FX[(exchangeratesapi.io)]

  end

  U --> A

  A --> AE --> IDEM

  %% Common read paths

  A -->|GET last| Q

  A -->|GET by id| QU

  %% DB mode

  A -->|POST /updates| QU

  WDB -->|claim queued| QU

  WDB -->|fetch| FX

  WDB -->|append| QH

  WDB -->|upsert| Q

  WDB -->|status done/failed| QU

  %% gRPC mode

  A -. 202 & bg goroutine .-> BG

  BG -->|gRPC Fetch| WGRPC

  WGRPC --> FX

  BG -->|append| QH

  BG -->|upsert| Q

  BG -->|status done/failed| QU

  %% chan mode (in-process)

  A -. enqueue .-> BG

  BG -->|fetch| FX

  BG -->|append| QH

  BG -->|upsert| Q

  BG -->|status done/failed| QU
```

Sequence Flows

A) DB mode (WORKER_TYPE=db)

```mermaid
sequenceDiagram

  participant C as Client

  participant API

  participant R as Redis (idempotency)

  participant PG as Postgres

  participant W as DbWorker

  participant FX as External API

  C->>API: POST /quotes/updates {pair} (X-Idempotency-Key)

  API->>R: SETNX key TTL

  alt not reserved

    API-->>C: 409 Conflict

  else reserved

    API->>PG: INSERT quote_updates(status='queued')

    API-->>C: 202 Accepted {update_id}

    W->>PG: SELECT ... FOR UPDATE SKIP LOCKED

    W->>FX: fetch(pair)

    W->>PG: INSERT quotes_history; UPSERT quotes

    W->>PG: UPDATE quote_updates(status='done' or 'failed')

  end

  C->>API: GET /quotes/updates/{id}

  API->>PG: SELECT status,(price,time via history join)

  API-->>C: 200 {status, price?, updated_at?}
```

B) gRPC mode (WORKER_TYPE=grpc)

```mermaid
sequenceDiagram

  participant C as Client

  participant API

  participant R as Redis

  participant PG as Postgres

  participant G as gRPC RateServer

  participant FX as External API

  C->>API: POST /quotes/updates {pair} (X-Idempotency-Key)

  API->>R: SETNX key TTL

  API->>PG: INSERT quote_updates(status='queued')

  API-->>C: 202 Accepted {update_id}

  par background goroutine

    API->>G: Fetch(pair, trace_id, timeout)

    G->>FX: fetch(pair)

    API->>PG: INSERT quotes_history; UPSERT quotes

    API->>PG: UPDATE quote_updates(status='done' or 'failed')

  end

  C->>API: GET /quotes/updates/{id}

  API->>PG: SELECT status,(price,time via history join)

  API-->>C: 200 {status, price?, updated_at?}
```

C) chan mode (WORKER_TYPE=chan)

```mermaid
sequenceDiagram

  participant C as Client

  participant API

  participant CH as in-proc channel pool

  participant FX as External API

  participant PG as Postgres

  C->>API: POST /quotes/updates {pair} (X-Idempotency-Key)

  API->>PG: INSERT quote_updates(status='queued')

  API->>CH: try send {update_id, pair, trace_id}

  alt queue full

    API-->>C: 503 queue busy

  else enqueued

    API-->>C: 202 Accepted {update_id}

    CH->>FX: fetch(pair)

    CH->>PG: INSERT quotes_history; UPSERT quotes

    CH->>PG: UPDATE quote_updates(status='done' or 'failed')

  end
```

Deployment (docker-compose)

```mermaid
graph TD

  subgraph docker

    P[postgres:17]

    D[redis:7]

    A[api (WORKER_TYPE=grpc|db|chan)]

    W1[worker #1]

    W2[worker #2]

    W3[worker #3]

  end

  A --- P

  A --- D

  W1 --- P

  W2 --- P

  W3 --- P

  classDef gray fill:#f6f8fa,stroke:#d0d7de,color:#24292f

  class P,D,A,W1,W2,W3 gray
```

Key Tables

	•	quotes(pair PK, price, updated_at) — latest rate per pair

	•	quote_updates(id PK, pair, status queued|processing|done|failed, error?, requested_at, completed_at?)

	•	quotes_history(id PK, pair, price, quoted_at, source, update_id→quote_updates.id, inserted_at) (unique on pair+quoted_at+source)

Notes

	•	Idempotency via X-Idempotency-Key (Redis SETNX + TTL).

	•	GET /quotes/updates/{id} returns status; if done, includes price and updated_at.

	•	gRPC and chan modes are asynchronous from the client’s perspective (HTTP returns 202).


