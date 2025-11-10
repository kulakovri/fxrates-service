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
	w, cleanup, err := bootstrap.InitWorker(ctx)
	if err != nil {
		log.Fatal("init worker", zap.Error(err))
	}
	defer cleanup()
	if w == nil {
		log.Fatal("no worker configured (WORKER_TYPE)", zap.Error(err))
	}
	w.Start(ctx)
}
