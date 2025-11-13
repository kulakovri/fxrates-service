# FXRates Service – Architecture Mapping

| **Service / Worker** | **Use Case / Function** | **Ports Used** | **Infrastructure Implementations** |
|-----------------------|-------------------------|----------------|------------------------------------|
| **HTTP API** | `RequestQuoteUpdate(ctx, pair, idem)` | `IdempotencyStore.TryReserve`, `UpdateJobRepo.CreateQueued` | Redis (`SETNX + TTL`), PG `update_job_repo` |
|  | `GetQuoteUpdate(ctx, id)` | `UpdateJobRepo.GetByID` | PG `update_job_repo` |
|  | `GetLastQuote(ctx, pair)` | `QuoteRepo.GetLast` | PG `quote_repo` |
|  | (Background path) `CompleteQuoteUpdate(ctx, updateID, fetch, "grpc")` | `UoW.Do`, `QuoteRepo.Upsert`, `QuoteRepo.AppendHistory`, `UpdateJobRepo.UpdateStatus` | PG `unit_of_work`, PG `quote_repo`, PG `update_job_repo` |
| **gRPC RateServer** | `FetchQuote(ctx, pair)` | `RateProvider.Get` | HTTP provider (`exchangeratesapi.io`) or `fake` (tests) |
| **ChanWorker (in-proc)** | `CompleteQuoteUpdate(ctx, updateID, fetch=FetchQuote, source="chan")` | `UoW.Do`, `QuoteRepo.Upsert`, `QuoteRepo.AppendHistory`, `UpdateJobRepo.UpdateStatus` | PG `unit_of_work`, PG `quote_repo`, PG `update_job_repo` |
| **DbWorker (separate process)** | `ProcessQueueBatch(ctx, limit)` → uses `CompleteQuoteUpdate` internally | `UpdateJobRepo.ClaimQueued`, `RateProvider.Get`, `UoW.Do`, `QuoteRepo.Upsert`, `QuoteRepo.AppendHistory`, `UpdateJobRepo.UpdateStatus` | PG `update_job_repo`, HTTP provider, PG `unit_of_work`, PG `quote_repo` |
| **Shared FXRatesService (core)** | `FetchQuote(ctx, pair)` | `RateProvider.Get` | HTTP provider (`exchangeratesapi.io`) |
|  | `CompleteQuoteUpdate(ctx, updateID, fetch func, source)` | `UoW.Do`, `QuoteRepo.Upsert`, `QuoteRepo.AppendHistory`, `UpdateJobRepo.UpdateStatus` | PG `unit_of_work`, PG `quote_repo`, PG `update_job_repo` |
|  | `ProcessQueueBatch(ctx, limit)` | `UpdateJobRepo.ClaimQueued` + calls above | PG `update_job_repo` |
|  | `RequestQuoteUpdate`, `GetQuoteUpdate`, `GetLastQuote` | As above | Redis, PG repos |

---

### Legend

- **Ports** → Interfaces defined in `internal/application/ports.go`
- **Infrastructure** → Implementations under `internal/infrastructure/*`
- **Service/Worker** → Entry point or adapter using the service
- **UoW.Do** → Unit of Work ensures atomic DB writes across `Upsert`, `AppendHistory`, `UpdateStatus`