package main

import (
	"context"

	"fxrates-service/internal/bootstrap"
	"fxrates-service/internal/config"
	"fxrates-service/internal/infrastructure/logx"
	"go.uber.org/zap"
)

func main() {
	log := logx.L()
	ctx := context.Background()
	switch config.Load().WorkerType {
	case "grpc":
		run, cleanup, err := bootstrap.InitGRPCRunner(ctx)
		if err != nil {
			log.Fatal("init grpc runner", zap.Error(err))
		}
		defer cleanup()
		if err := run(ctx); err != nil {
			log.Fatal("grpc worker server exited", zap.Error(err))
		}
	default:
		w, cleanup, err := bootstrap.InitDBWorker(ctx)
		if err != nil {
			log.Fatal("init db worker", zap.Error(err))
		}
		defer cleanup()
		if w == nil {
			log.Fatal("no worker configured (WORKER_TYPE)", zap.Error(err))
		}
		w.Start(ctx)
	}
}
