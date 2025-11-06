package application

import (
	"context"

	"fxrates-service/internal/domain"
)

type FXRatesService struct {
	quoteRepo     QuoteRepo
	updateJobRepo UpdateJobRepo
	rateProvider  RateProvider
	clock         Clock
	idgen         IDGen
}

type Option func(*FXRatesService)

func WithClock(c Clock) Option { return func(s *FXRatesService) { s.clock = c } }
func WithIDGen(g IDGen) Option { return func(s *FXRatesService) { s.idgen = g } }

func NewFXRatesService(quoteRepo QuoteRepo, updateJobRepo UpdateJobRepo, rateProvider RateProvider, opts ...Option) *FXRatesService {
	s := &FXRatesService{
		quoteRepo:     quoteRepo,
		updateJobRepo: updateJobRepo,
		rateProvider:  rateProvider,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.clock == nil {
		s.clock = realClock{}
	}
	if s.idgen == nil {
		s.idgen = defaultIDGen{}
	}
	return s
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
