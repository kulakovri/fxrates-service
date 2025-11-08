package provider

import (
	"context"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
)

// Ensure Fake implements application.RateProvider.
var _ application.RateProvider = (*Fake)(nil)

type Fake struct {
	price float64
}

func NewFake(price float64) *Fake { return &Fake{price: price} }

func (f *Fake) Get(_ context.Context, pair string) (domain.Quote, error) {
	return domain.Quote{
		Pair:      domain.Pair(pair),
		Price:     f.price,
		UpdatedAt: time.Now().UTC(),
	}, nil
}
