package application

import (
	"context"

	"fxrates-service/internal/domain"
)

type QuoteRepo interface {
	GetLast(ctx context.Context, pair string) (domain.Quote, error)
	Upsert(ctx context.Context, q domain.Quote) error
	AppendHistory(ctx context.Context, q domain.QuoteHistory) error
}

type UpdateJobRepo interface {
	CreateQueued(ctx context.Context, pair string, idem *string) (string, error)
	GetByID(ctx context.Context, id string) (domain.QuoteUpdate, error)
	UpdateStatus(ctx context.Context, id string, status domain.QuoteUpdateStatus, errMsg *string) error
}

type RateProvider interface {
	Get(ctx context.Context, pair string) (domain.Quote, error)
}
