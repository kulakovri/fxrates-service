package bootstrap

import (
	"context"
	"fmt"

	"fxrates-service/internal/config"
)

type WorkerApp func(ctx context.Context) error

func InitWorkerApp(ctx context.Context) (WorkerApp, func(), error) {
	cfg := config.Load()

	switch cfg.WorkerType {
	case "grpc":
		run, cleanup, err := InitGRPCRunner(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("init grpc runner: %w", err)
		}
		return run, cleanup, nil

	case "", "db":
		w, cleanup, err := InitDBWorker(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("init db worker: %w", err)
		}
		if w == nil {
			return nil, nil, fmt.Errorf("no worker configured for WORKER_TYPE=%q", cfg.WorkerType)
		}
		runner := func(ctx context.Context) error {
			w.Start(ctx)
			return nil
		}
		return runner, cleanup, nil

	default:
		return nil, nil, fmt.Errorf("unsupported WORKER_TYPE=%q", cfg.WorkerType)
	}
}
