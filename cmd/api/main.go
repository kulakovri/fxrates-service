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

func main() {
	ctx := context.Background()
	logger := logx.L()

	// Configuration
	port := ":8080"
	env := "dev"
	serviceName := "fxrates-service"

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
		Addr:    port,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting HTTP server",
			zap.String("service", serviceName),
			zap.String("env", env),
			zap.String("port", port),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
}
