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

// Worker injector: builds application.Worker or gRPC server runner + Cleanup
func InitWorker(ctx context.Context) (application.Worker, func(context.Context) error, func(), error) {
	wire.Build(
		infraSet,
		ProvideWorker,
		ProvideGRPCRateServerRunner,
	)
	return nil, nil, nil, nil
}
