# fxrates-service

Asynchronous FX rates microservice in Go 1.22 (Clean Architecture).

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

## Endpoints

| Method | Path                         | Description         |
|-------|------------------------------|---------------------|
| GET   | /healthz                     | liveness probe      |
| GET   | /readyz                      | readiness probe     |
| POST  | /quotes/updates              | queue update        |
| GET   | /quotes/updates/{id}         | check status        |
| GET   | /quotes/last?pair=EUR/USD    | get last quote      |

