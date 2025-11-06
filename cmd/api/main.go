package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpserver "fxrates-service/internal/infrastructure/http"
	"fxrates-service/internal/infrastructure/logx"
	"fxrates-service/internal/infrastructure/worker"

	"go.uber.org/zap"
)

type Config struct {
	Port     string
	LogLevel string
	Env      string
}

func loadConfig() Config {
	return Config{
		Port:     getenv("PORT", "8080"),
		LogLevel: getenv("LOG_LEVEL", "info"),
		Env:      getenv("ENV", "local"),
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

	// Setup HTTP server via generated router
	svc, quoteRepo, jobRepo, provider := httpserver.NewInMemoryService()
	srv := httpserver.NewServer(svc)
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
