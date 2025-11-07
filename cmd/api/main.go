package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fxrates-service/internal/application"
	httpserver "fxrates-service/internal/infrastructure/http"
	"fxrates-service/internal/infrastructure/logx"
	"fxrates-service/internal/infrastructure/pg"
	"fxrates-service/internal/infrastructure/worker"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func init() { _ = godotenv.Load() }

type Config struct {
	Port        string
	LogLevel    string
	Env         string
	Storage     string
	DatabaseURL string
}

func loadConfig() Config {
	return Config{
		Port:        getenv("PORT", "8080"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		Env:         getenv("ENV", "local"),
		Storage:     getenv("STORAGE", "inmem"),
		DatabaseURL: getenv("DATABASE_URL", ""),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	ctx := context.Background()
	logger := logx.L()
	cfg := loadConfig()
	addr := ":" + cfg.Port

	// Setup storage
	var (
		quoteRepo application.QuoteRepo
		jobRepo   application.UpdateJobRepo
		provider  application.RateProvider
	)
	var pingFn func(context.Context) error
	if cfg.Storage == "pg" {
		db, err := pg.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Fatal("pg connect", zap.Error(err))
		}
		if err := pg.RunMigrations(ctx, db); err != nil {
			logger.Fatal("migrate", zap.Error(err))
		}
		quoteRepo = pg.NewQuoteRepo(db)
		jobRepo = pg.NewUpdateJobRepo(db)
		provider = httpserver.NewFakeRateProvider()
		pingFn = db.Ping
	} else {
		quoteRepo, jobRepo, provider = httpserver.NewInMemoryRepos()
	}

	svc := application.NewFXRatesService(quoteRepo, jobRepo, provider)
	srv := httpserver.NewServer(svc)
	if pingFn != nil {
		srv.SetReadyCheck(pingFn)
	}
	mux := httpserver.NewRouter(srv)

	// Start in-memory worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &worker.InMemWorker{Updates: jobRepo, Quotes: quoteRepo, Provider: provider, PollEvery: 500 * time.Millisecond}
	go w.Start(ctx)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server started", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shutdownCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = server.Shutdown(shutdownCtx)
	logger.Info("server stopped")
}
