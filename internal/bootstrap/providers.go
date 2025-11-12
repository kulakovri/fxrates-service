package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/config"
	rateclient "fxrates-service/internal/infrastructure/grpc/rateclient"
	grpcserver "fxrates-service/internal/infrastructure/grpc/rateserver"
	"fxrates-service/internal/infrastructure/httpx"
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

// helpers (used locally where necessary)
func atoiDef(s string, def int) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}

func ProvideLogger() *zap.Logger { return logx.L() }

func ProvideConfig() config.Config { return config.Load() }

type ChanBus struct {
	Ch       chan worker.UpdateMsg
	Enqueue  func(ctx context.Context, id, pair, traceID string) error
	Shutdown func()
}

func ProvideChanBus(cfg config.Config) *ChanBus {
	qSize := cfg.ChanQueueSize
	ch := make(chan worker.UpdateMsg, qSize)
	return &ChanBus{
		Ch: ch,
		Enqueue: func(ctx context.Context, id, pair, traceID string) error {
			select {
			case ch <- worker.UpdateMsg{ID: id, Pair: pair, TraceID: traceID}:
				return nil
			default:
				t := time.NewTimer(50 * time.Millisecond)
				defer t.Stop()
				select {
				case ch <- worker.UpdateMsg{ID: id, Pair: pair, TraceID: traceID}:
					return nil
				case <-t.C:
					return fmt.Errorf("queue full")
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		},
		Shutdown: func() { close(ch) },
	}
}

func ProvideDB(ctx context.Context, log *zap.Logger, cfg config.Config) (*pg.DB, func(), error) {
	dbURL := cfg.DatabaseURL
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

func ProvideRedisClient(cfg config.Config) (*redis.Client, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return client, func() { _ = client.Close() }, nil
}

func ProvideIdempotency(client *redis.Client, cfg config.Config) Services {
	store := redisstore.New(client, cfg.RedisTTL)
	return Services{Idem: store}
}

// BuildCleanup is not needed when using wire's built-in cleanup aggregation.

func ProvideRateProvider(cfg config.Config) (application.RateProvider, error) {
	switch cfg.Provider {
	case "exchangeratesapi":
		return &provider.ExchangeRatesAPIProvider{
			BaseURL: cfg.ExchangeAPIBase,
			APIKey:  cfg.ExchangeAPIKey,
			Client:  &httpx.Client{HTTP: &http.Client{Timeout: 4 * time.Second}},
			BackoffCfg: &httpx.BackoffConfig{
				Initial: cfg.HTTPBackoffInitial,
				Max:     cfg.HTTPBackoffMax,
				Total:   cfg.HTTPBackoffTotal,
			},
		}, nil
	default:
		return provider.NewFake(1.2345), nil
	}
}

func ProvideFXRatesService(r Repos, rp application.RateProvider, s Services) *application.FXRatesService {
	return application.NewService(r.QuoteRepo, r.JobRepo, rp, s.Idem)
}

// ProvideGRPCRateClient optionally dials the worker gRPC when WORKER_TYPE=grpc.
func ProvideGRPCRateClient(cfg config.Config) (*rateclient.Client, func(), error) {
	if cfg.WorkerType != "grpc" {
		return nil, func() {}, nil
	}
	ctx := context.Background()
	c, cleanup, err := rateclient.New(ctx, cfg.GRPCTarget)
	if err != nil {
		return nil, func() {}, err
	}
	return c, cleanup, nil
}

// ProvideGRPCRateServerRunner returns a runner to start the gRPC worker server when WORKER_TYPE=grpc.
// The bool indicates whether the runner is enabled.
func ProvideGRPCRateServerRunner(cfg config.Config, svc *application.FXRatesService, log *zap.Logger) (func(ctx context.Context) error, bool) {
	if cfg.WorkerType != "grpc" {
		return nil, false
	}
	addr := cfg.GRPCAddr
	return func(ctx context.Context) error {
		s := grpcserver.NewServer(svc, log)
		return grpcserver.RunServer(ctx, addr, s, log)
	}, true
}

func ProvideWorker(svc *application.FXRatesService, rp application.RateProvider, log *zap.Logger, cfg config.Config) application.Worker {
	switch cfg.WorkerType {
	case "db":
		return worker.NewDBWorker(svc, cfg.WorkerPoll, cfg.WorkerBatchSize, log)
	default:
		if log != nil {
			log.Error("unknown WORKER_TYPE; no worker launched")
		}
		return nil
	}
}

// Two-arg combiner for Wire to inject both cleanups (PG, Redis)
// (no longer needed; wire aggregates cleanup funcs automatically)
