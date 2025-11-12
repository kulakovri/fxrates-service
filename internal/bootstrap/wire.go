//go:build wireinject

package bootstrap

import (
	"context"

	"fxrates-service/internal/application"
	httpserver "fxrates-service/internal/infrastructure/http"

	"github.com/google/wire"
)

var infraSet = wire.NewSet(
	ProvideLogger,
	ProvideConfig,
	ProvideDB,
	ProvideRepos,
	ProvideUoW,
	ProvideRedisClient,
	ProvideIdempotency,
	ProvideRateProvider,
	ProvideFXRatesService,
	ProvideGRPCRateClient,
)

// API injector: builds *httpserver.Server + Cleanup
func InitAPI(ctx context.Context) (*httpserver.Server, func(), error) {
	wire.Build(
		infraSet,
		httpserver.NewServer,
	)
	return nil, nil, nil
}

// DB Worker injector: builds application.Worker + Cleanup
func InitDBWorker(ctx context.Context) (application.Worker, func(), error) {
	wire.Build(
		infraSet,
		ProvideWorker,
	)
	return nil, nil, nil
}

// gRPC Runner injector: builds gRPC server runner + Cleanup
func InitGRPCRunner(ctx context.Context) (func(context.Context) error, func(), error) {
	wire.Build(
		infraSet,
		ProvideGRPCRateServerRunner,
	)
	return nil, nil, nil
}
