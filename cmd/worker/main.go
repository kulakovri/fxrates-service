package main

import (
	"context"

	"fxrates-service/internal/bootstrap"
	"fxrates-service/internal/infrastructure/logx"
	"go.uber.org/zap"
)

func main() {
	log := logx.L()
	ctx := context.Background()
	run, cleanup, err := bootstrap.InitWorkerApp(ctx)
	if err != nil {
		log.Fatal("init worker app", zap.Error(err))
	}
	defer cleanup()
	if err := run(ctx); err != nil {
		log.Fatal("worker exited", zap.Error(err))
	}
}
