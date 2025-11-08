package application

import (
	"context"
	"time"

	"fxrates-service/internal/domain"

	"github.com/google/uuid"
)

// Functional seams for time and ID generation
type ClockFunc func() time.Time
type IDGenFunc func() string

// Option allows injecting behavior into FXRatesService
type Option func(*FXRatesService)

type FXRatesService struct {
	quoteRepo     QuoteRepo
	updateJobRepo UpdateJobRepo
	rateProvider  RateProvider

	now   ClockFunc
	newID IDGenFunc
	idem  IdempotencyStore
}

func WithClock(f ClockFunc) Option { return func(s *FXRatesService) { s.now = f } }
func WithIDGen(f IDGenFunc) Option { return func(s *FXRatesService) { s.newID = f } }

func NewFXRatesService(quoteRepo QuoteRepo, updateJobRepo UpdateJobRepo, rateProvider RateProvider, idem IdempotencyStore, opts ...Option) *FXRatesService {
	s := &FXRatesService{
		quoteRepo:     quoteRepo,
		updateJobRepo: updateJobRepo,
		rateProvider:  rateProvider,
		now:           time.Now,
		newID:         func() string { return uuid.NewString() },
	}
	if idem != nil {
		s.idem = idem
	} else {
		s.idem = NoopIdempotency{}
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *FXRatesService) RequestQuoteUpdate(ctx context.Context, pair string, idem *string) (string, error) {
	if idem == nil || *idem == "" {
		return "", ErrBadRequest
	}
	ok, err := s.idem.TryReserve(ctx, *idem)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrConflict
	}
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
