package main

import (
	"context"
	"os/signal"
	"syscall"

	"fxrates-service/internal/bootstrap"
	"fxrates-service/internal/infrastructure/logx"
	"go.uber.org/zap"
)

func main() {
	log := logx.L()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repos, cleanup, err := bootstrap.BuildRepos(ctx)
	if err != nil {
		log.Fatal("bootstrap repos", zap.Error(err))
	}
	defer cleanup()

	w := bootstrap.BuildWorker(repos)
	if w == nil {
		log.Fatal("no worker configured (WORKER_TYPE)")
	}

	log.Info("worker starting")
	w.Start(ctx)
	log.Info("worker stopped")
}
