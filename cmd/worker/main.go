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
	w, run, cleanup, err := bootstrap.InitWorker(ctx)
	if err != nil {
		log.Fatal("init worker", zap.Error(err))
	}
	defer cleanup()
	switch {
	case run != nil:
		if err := run(ctx); err != nil {
			log.Fatal("grpc worker server exited", zap.Error(err))
		}
	case w != nil:
		w.Start(ctx)
	default:
		log.Fatal("no worker configured (WORKER_TYPE)", zap.Error(err))
	}
}
