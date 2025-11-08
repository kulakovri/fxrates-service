package bootstrap

import (
	"context"
	"fmt"
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
)

type Repos struct {
	QuoteRepo application.QuoteRepo
	JobRepo   application.UpdateJobRepo
}

type Services struct {
	Idem application.IdempotencyStore
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiDef(s string, def int) int {
	i, err := strconv.Atoi(s)
	if err != nil || i <= 0 {
		return def
	}
	return i
}

func durMS(key string, defMS int) time.Duration {
	ms := atoiDef(getenv(key, fmt.Sprint(defMS)), defMS)
	return time.Duration(ms) * time.Millisecond
}

// BuildRepos builds repositories based on STORAGE env ("pg" expected).
func BuildRepos(ctx context.Context) (Repos, func(), error) {
	log := logx.L()
	storage := getenv("STORAGE", "pg")

	switch storage {
	case "pg":
		dbURL := getenv("DATABASE_URL", "")
		if dbURL == "" {
			return Repos{}, func() {}, fmt.Errorf("DATABASE_URL is required for STORAGE=pg")
		}
		db, err := pg.Connect(ctx, dbURL)
		if err != nil {
			return Repos{}, func() {}, err
		}
		if err := pg.RunMigrations(ctx, db); err != nil {
			db.Close()
			return Repos{}, func() {}, err
		}
		quoteRepo := pg.NewQuoteRepo(db)
		jobRepo := pg.NewUpdateJobRepo(db)
		cleanup := func() {
			log.Info("closing pg")
			db.Close()
		}
		return Repos{QuoteRepo: quoteRepo, JobRepo: jobRepo}, cleanup, nil
	default:
		return Repos{}, func() {}, fmt.Errorf("in-memory repos not implemented via bootstrap; set STORAGE=pg")
	}
}

// BuildRateProvider returns a provider instance (fake for now).
func BuildRateProvider() application.RateProvider {
	switch getenv("PROVIDER", "fake") {
	case "http":
		// TODO: add real HTTP provider when ready
		return provider.NewFake(1.2345)
	default:
		return provider.NewFake(1.2345)
	}
}

// BuildWorker constructs an application.Worker based on WORKER_TYPE env.
func BuildWorker(repos Repos) application.Worker {
	log := logx.L()
	switch getenv("WORKER_TYPE", "db") {
	case "db":
		return &worker.DbWorker{
			Jobs:       repos.JobRepo,
			Quotes:     repos.QuoteRepo,
			Provider:   BuildRateProvider(),
			PollEvery:  durMS("WORKER_POLL_MS", 250),
			BatchLimit: atoiDef(getenv("WORKER_BATCH_LIMIT", "10"), 10),
			Log:        log,
		}
	default:
		return nil
	}
}

// BuildRedis builds the idempotency store if enabled (defaults to redis; falls back to Noop).
func BuildRedis() (Services, func(), error) {
	use := getenv("IDEMPOTENCY_BACKEND", "redis") // or "none"
	if use != "redis" {
		return Services{Idem: application.NoopIdempotency{}}, func() {}, nil
	}
	addr := getenv("REDIS_ADDR", "localhost:6379")
	pass := getenv("REDIS_PASSWORD", "")
	db := atoiDef(getenv("REDIS_DB", "0"), 0)
	ttl := durMS("IDEMPOTENCY_TTL_MS", 24*60*60*1000)
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: pass, DB: db})
	store := redisstore.New(rdb, ttl)
	cleanup := func() { _ = rdb.Close() }
	return Services{Idem: store}, cleanup, nil
}
