# Clean Architecture (Layered View)

Dependencies flow **inward only**:

Infrastructure ⟶ Application (Ports/Use Cases) ⟶ Domain (Entities/Value Objects).

## Concentric Layers (radial approximation)

```mermaid

flowchart TB

  %% We simulate concentric rings with nested subgraphs and one-way arrows.

  subgraph L3[Infrastructure outer ring]

    direction TB

    %% Adapters

    HTTP[HTTP Adapter\nchi + OpenAPI]

    PG[Postgres Adapter\npgx repos + migrations]

    REDIS[Redis Adapter\nIdempotencyStore]

    GRPC_SRV[gRPC RateServer]

    GRPC_CLI[gRPC Client]

    PROVIDER[FX Provider\nexchangeratesapi.io via httpx]

    WORKER_DB[DbWorker\npoll + SKIP LOCKED]

    WORKER_CH[ChanWorker\nin-proc pool]

    subgraph L2[Application use cases and ports]

      direction TB

      SVC[FXRatesService\nRequestQuoteUpdate\nGetQuoteUpdate\nGetLastQuote\nProcessGRPCUpdate]

      %% Ports

      portQ[Port: QuoteRepo]

      portU[Port: UpdateJobRepo]

      portI[Port: IdempotencyStore]

      portP[Port: RateProvider]

      subgraph L1[Domain center]

        direction TB

        ENT[Entities\nPair, Quote, QuoteUpdate, QuoteHistory]

        VO[Rules\nValidatePair, Status enum]

      end

    end

  end

  %% Allowed directions (outer -> inner)

  HTTP --> SVC

  GRPC_CLI --> SVC

  WORKER_DB --> SVC

  WORKER_CH --> SVC

  %% Adapters implement ports (deps point inward)

  PG --> portQ

  PG --> portU

  REDIS --> portI

  PROVIDER --> portP

  GRPC_SRV --> PROVIDER

  %% Use cases depend on ports and domain

  SVC --> portQ

  SVC --> portU

  SVC --> portI

  SVC --> portP

  SVC --> ENT

  SVC --> VO

  %% Styling to suggest rings

  classDef ring1 fill:#fdfdfd,stroke:#c9d1d9,color:#24292f

  classDef ring2 fill:#f6f8fa,stroke:#c9d1d9,color:#24292f

  classDef ring3 fill:#eef2f7,stroke:#c9d1d9,color:#24292f

  class L1 ring1

  class L2 ring2

  class L3 ring3
```

Ports and Adapters Matrix (quick map)

```mermaid
classDiagram

  class QuoteRepo

  class UpdateJobRepo

  class IdempotencyStore

  class RateProvider

  class PGRepos {

    +GetLast(pair)

    +Upsert(quote)

    +AppendHistory(history)

    +CreateQueued(pair, idem)

    +GetByID(id)

    +UpdateStatus(id, status, err)

    +ClaimQueued(limit)

  }

  class RedisIdem { +TryReserve(key, ttl) }

  class XRatesProvider { +Get(ctx, pair) }

  class GRPCRateServer { +Fetch(pair, trace, timeout) }

  QuoteRepo <|.. PGRepos

  UpdateJobRepo <|.. PGRepos

  IdempotencyStore <|.. RedisIdem

  RateProvider <|.. XRatesProvider

  GRPCRateServer ..> XRatesProvider : uses
```

Runtime Modes

	•	WORKER_TYPE=db — external DbWorker claims queued jobs from Postgres and writes results back.

	•	WORKER_TYPE=grpc — API returns 202 and runs a background call to gRPC RateServer; API persists results.

	•	WORKER_TYPE=chan — API enqueues into an in-process channel pool; workers fetch rates and persist.

Common read paths:

	•	GET /quotes/last?pair=X/Y reads quotes.

	•	GET /quotes/updates/{id} reads quote_updates and joins quotes_history for price/time.

In `README.md`, add under any “Documentation” or “Project Docs” section:

```md

- [Clean Architecture (layers)](docs/CLEAN_ARCH.md)

⸻

This gives you the “domain-at-center” feel while keeping it Mermaid-only (works great in GitHub/Renderers). If you later want a true circular visual, we can also export a small SVG via a draw.io source, but this keeps it simple and versionable.

