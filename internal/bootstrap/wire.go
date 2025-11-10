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
	ProvideDB,
	ProvideRepos,
	ProvideRedisClient,
	ProvideIdempotency,
	ProvideRateProvider,
	ProvideFXRatesService,
)

// API injector: builds *httpserver.Server + Cleanup
func InitAPI(ctx context.Context) (*httpserver.Server, func(), error) {
	wire.Build(
		infraSet,
		httpserver.NewServer,
	)
	return nil, nil, nil
}

// Worker injector: builds application.Worker + Cleanup
func InitWorker(ctx context.Context) (application.Worker, func(), error) {
	wire.Build(
		infraSet,
		ProvideWorker,
	)
	return nil, nil, nil
}
