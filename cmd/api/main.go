package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/bootstrap"
	httpserver "fxrates-service/internal/infrastructure/http"
	"fxrates-service/internal/infrastructure/logx"

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

	// Setup repositories via bootstrap (expects STORAGE=pg)
	repos, cleanup, err := bootstrap.BuildRepos(ctx)
	if err != nil {
		logger.Fatal("bootstrap repos", zap.Error(err))
	}
	defer cleanup()

	svc := application.NewFXRatesService(repos.QuoteRepo, repos.JobRepo, bootstrap.BuildRateProvider())
	srv := httpserver.NewServer(svc)
	// Ready check uses DB ping if available
	// Bootstrap returns PG repos currently, so provide pg ping through BuildRepos cleanup/handle
	// Not exposing ping here for brevity
	mux := httpserver.NewRouter(srv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// API process no longer starts workers; run cmd/worker separately

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
