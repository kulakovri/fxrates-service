package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/config"
	"fxrates-service/internal/infrastructure/logx"
	"fxrates-service/internal/infrastructure/pg"
	"fxrates-service/internal/infrastructure/provider"
	redisstore "fxrates-service/internal/infrastructure/redis"
	"fxrates-service/internal/infrastructure/worker"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Repos struct {
	QuoteRepo application.QuoteRepo
	JobRepo   application.UpdateJobRepo
}

type Services struct {
	Idem application.IdempotencyStore
}

// BuildRepos builds repositories based on STORAGE env ("pg" expected).
func BuildRepos(ctx context.Context, cfg config.Config) (Repos, func(), error) {
	log := logx.L()
	storage := cfg.Storage

	switch storage {
	case "pg":
		dbURL := cfg.DatabaseURL
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

// BuildRateProvider returns a provider instance based on env configuration.
func BuildRateProvider(cfg config.Config) (application.RateProvider, error) {
	switch cfg.Provider {
	case "exchangeratesapi":
		base := cfg.ExchangeAPIBase
		key := cfg.ExchangeAPIKey
		client := &http.Client{Timeout: 4 * time.Second}
		return &provider.ExchangeRatesAPIProvider{
			BaseURL: base,
			APIKey:  key,
			Client:  client,
		}, nil
	default:
		return provider.NewFake(1.2345), nil
	}
}

// BuildWorker constructs an application.Worker based on WORKER_TYPE env.
func BuildWorker(cfg config.Config, repos Repos) application.Worker {
	log := logx.L()
	switch cfg.WorkerType {
	case "db":
		rateProvider, err := BuildRateProvider(cfg)
		if err != nil {
			log.Error("failed to build rate provider", zap.Error(err))
			return nil
		}
		return &worker.DbWorker{
			Jobs:       repos.JobRepo,
			Quotes:     repos.QuoteRepo,
			Provider:   rateProvider,
			PollEvery:  cfg.WorkerPoll,
			BatchLimit: cfg.WorkerBatchSize,
			Log:        log,
		}
	default:
		return nil
	}
}

// BuildRedis builds the idempotency store if enabled (defaults to redis; falls back to Noop).
func BuildRedis(cfg config.Config) (Services, func(), error) {
	addr := cfg.RedisAddr
	pass := cfg.RedisPassword
	db := cfg.RedisDB
	ttl := cfg.RedisTTL
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: pass, DB: db})
	store := redisstore.New(rdb, ttl)
	cleanup := func() { _ = rdb.Close() }
	return Services{Idem: store}, cleanup, nil
}
