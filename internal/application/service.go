package application

import (
	"context"

	"fxrates-service/internal/domain"
)

type FXRatesService struct {
	quoteRepo     QuoteRepo
	updateJobRepo UpdateJobRepo
	rateProvider  RateProvider
}

func NewFXRatesService(quoteRepo QuoteRepo, updateJobRepo UpdateJobRepo, rateProvider RateProvider) *FXRatesService {
	return &FXRatesService{
		quoteRepo:     quoteRepo,
		updateJobRepo: updateJobRepo,
		rateProvider:  rateProvider,
	}
}

func (s *FXRatesService) RequestQuoteUpdate(ctx context.Context, pair string, idem *string) (string, error) {
	updateID, err := s.updateJobRepo.CreateQueued(ctx, pair, idem)
	if err != nil {
		return "", err
	}
	return updateID, nil
}

func (s *FXRatesService) GetQuoteUpdate(ctx context.Context, id string) (domain.QuoteUpdate, error) {
	return s.updateJobRepo.GetByID(ctx, id)
}

func (s *FXRatesService) GetLastQuote(ctx context.Context, pair string) (domain.Quote, error) {
	return s.quoteRepo.GetLast(ctx, pair)
}
