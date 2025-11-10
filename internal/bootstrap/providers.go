package bootstrap

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/infrastructure/logx"
	"fxrates-service/internal/infrastructure/pg"
	"fxrates-service/internal/infrastructure/provider"
	redisstore "fxrates-service/internal/infrastructure/redis"
	"fxrates-service/internal/infrastructure/worker"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// (no explicit Cleanup type needed; providers return func() for wire aggregation)

type Repos struct {
	QuoteRepo application.QuoteRepo
	JobRepo   application.UpdateJobRepo
}

type Services struct {
	Idem application.IdempotencyStore
}

// helpers
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiDef(s string, def int) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}

func durMS(env string, defMs int) time.Duration {
	return time.Duration(atoiDef(getenv(env, strconv.Itoa(defMs)), defMs)) * time.Millisecond
}

func ProvideLogger() *zap.Logger { return logx.L() }

func ProvideDB(ctx context.Context, log *zap.Logger) (*pg.DB, func(), error) {
	dbURL := getenv("DATABASE_URL", "")
	if dbURL == "" {
		return nil, func() {}, ErrMissingDBURL
	}
	db, err := pg.Connect(ctx, dbURL)
	if err != nil {
		return nil, func() {}, err
	}
	if err := pg.RunMigrations(ctx, db); err != nil {
		db.Close()
		return nil, func() {}, err
	}
	cleanup := func() {
		if log != nil {
			log.Info("closing pg")
		}
		db.Close()
	}
	return db, cleanup, nil
}

func ProvideRepos(db *pg.DB) Repos {
	return Repos{
		QuoteRepo: pg.NewQuoteRepo(db),
		JobRepo:   pg.NewUpdateJobRepo(db),
	}
}

func ProvideRedisClient() (*redis.Client, func(), error) {
	addr := getenv("REDIS_ADDR", "localhost:6379")
	pass := getenv("REDIS_PASSWORD", "")
	rdb := atoiDef(getenv("REDIS_DB", "0"), 0)
	client := redis.NewClient(&redis.Options{Addr: addr, Password: pass, DB: rdb})
	return client, func() { _ = client.Close() }, nil
}

func ProvideIdempotency(client *redis.Client) Services {
	ttl := durMS("IDEMPOTENCY_TTL_MS", 24*60*60*1000)
	store := redisstore.New(client, ttl)
	return Services{Idem: store}
}

// BuildCleanup is not needed when using wire's built-in cleanup aggregation.

func ProvideRateProvider() (application.RateProvider, error) {
	switch getenv("PROVIDER", "fake") {
	case "exchangeratesapi":
		base := getenv("EXCHANGE_API_BASE", "https://api.exchangeratesapi.io")
		key := getenv("EXCHANGE_API_KEY", "")
		return &provider.ExchangeRatesAPIProvider{
			BaseURL: base,
			APIKey:  key,
			Client:  &http.Client{Timeout: 4 * time.Second},
		}, nil
	default:
		return provider.NewFake(1.2345), nil
	}
}

func ProvideFXRatesService(r Repos, rp application.RateProvider, s Services) *application.FXRatesService {
	return application.NewFXRatesService(r.QuoteRepo, r.JobRepo, rp, s.Idem)
}

func ProvideWorker(r Repos, rp application.RateProvider, log *zap.Logger) application.Worker {
	switch getenv("WORKER_TYPE", "db") {
	case "db":
		return &worker.DbWorker{
			Jobs:       r.JobRepo,
			Quotes:     r.QuoteRepo,
			Provider:   rp,
			PollEvery:  durMS("WORKER_POLL_MS", 250),
			BatchLimit: atoiDef(getenv("WORKER_BATCH_LIMIT", "10"), 10),
			Log:        log,
		}
	default:
		if log != nil {
			log.Error("unknown WORKER_TYPE; no worker launched")
		}
		return nil
	}
}

// Two-arg combiner for Wire to inject both cleanups (PG, Redis)
// (no longer needed; wire aggregates cleanup funcs automatically)
