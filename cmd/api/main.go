package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"fxrates-service/internal/infrastructure/logx"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	logger := logx.L()

	// Configuration
	port := ":8080"
	env := "dev"
	serviceName := "fxrates-service"

	// Setup HTTP server
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

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
