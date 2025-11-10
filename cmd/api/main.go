package main

import (
	"context"

	"fxrates-service/internal/bootstrap"
	"fxrates-service/internal/infrastructure/logx"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func init() { _ = godotenv.Load() }

func main() {
	ctx := context.Background()
	log := logx.L()
	srv, cleanup, err := bootstrap.InitAPI(ctx)
	if err != nil {
		log.Fatal("init api", zap.Error(err))
	}
	defer cleanup()
	if err := srv.Run(ctx); err != nil {
		log.Fatal("server run", zap.Error(err))
	}
}
