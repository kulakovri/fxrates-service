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
	"fxrates-service/internal/config"
	httpserver "fxrates-service/internal/infrastructure/http"
	"fxrates-service/internal/infrastructure/logx"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func init() { _ = godotenv.Load() }

func main() {
	ctx := context.Background()
	logger := logx.L()
	cfg := config.Load()
	addr := ":" + cfg.Port

	// Setup repositories via bootstrap (expects STORAGE=pg)
	repos, cleanup, err := bootstrap.BuildRepos(ctx, cfg)
	if err != nil {
		logger.Fatal("bootstrap repos", zap.Error(err))
	}
	defer cleanup()

	services, closeRedis, err := bootstrap.BuildRedis(cfg)
	if err != nil {
		logger.Fatal("bootstrap redis", zap.Error(err))
	}
	defer closeRedis()
	rateProvider, err := bootstrap.BuildRateProvider(cfg)
	if err != nil {
		logger.Fatal("bootstrap rate provider", zap.Error(err))
	}
	svc := application.NewFXRatesService(repos.QuoteRepo, repos.JobRepo, rateProvider, services.Idem)
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
