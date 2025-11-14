package application

import (
	"context"
	"errors"
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
	uow           UnitOfWork

	now   ClockFunc
	newID IDGenFunc
	idem  IdempotencyStore
}

func WithClock(f ClockFunc) Option { return func(s *FXRatesService) { s.now = f } }
func WithIDGen(f IDGenFunc) Option { return func(s *FXRatesService) { s.newID = f } }
func WithUoW(u UnitOfWork) Option  { return func(s *FXRatesService) { s.uow = u } }

func NewService(quoteRepo QuoteRepo, updateJobRepo UpdateJobRepo, rateProvider RateProvider, idem IdempotencyStore, opts ...Option) *FXRatesService {
	s := &FXRatesService{
		quoteRepo:     quoteRepo,
		updateJobRepo: updateJobRepo,
		rateProvider:  rateProvider,
		uow:           NoopUoW{},
		now:           time.Now,
		newID:         func() string { return uuid.NewString() },
	}
	if idem != nil {
		s.idem = idem
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
	var ok bool
	var err error
	if s.idem != nil {
		ok, err = s.idem.TryReserve(ctx, *idem)
	} else {
		ok = true
	}
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
	upd, err := s.updateJobRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.QuoteUpdate{}, domain.ErrNotFound
		}
		return domain.QuoteUpdate{}, err
	}
	return upd, nil
}

func (s *FXRatesService) GetLastQuote(ctx context.Context, pair string) (domain.Quote, error) {
	q, err := s.quoteRepo.GetLast(ctx, pair)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Quote{}, domain.ErrNotFound
		}
		return domain.Quote{}, err
	}
	return q, nil
}

// QuoteFetcher is a small facade to fetch quotes via the service without exposing ports.
type QuoteFetcher interface {
	FetchQuote(ctx context.Context, pair string) (domain.Quote, error)
}

// FetchQuote delegates quote fetching to the provider.
func (s *FXRatesService) FetchQuote(ctx context.Context, pair string) (domain.Quote, error) {
	return s.rateProvider.Get(ctx, pair)
}

// CompleteQuoteUpdate performs background processing to fetch a quote and persist results.
// The fetch function abstracts the transport and must return a complete domain.Quote.
func (s *FXRatesService) CompleteQuoteUpdate(
	ctx context.Context,
	updateID string,
	fetch func(context.Context) (domain.Quote, error),
	source string,
) error {
	q, err := fetch(ctx)
	if err != nil {
		msg := err.Error()
		_ = s.updateJobRepo.UpdateStatus(ctx, updateID, domain.QuoteUpdateStatusFailed, &msg)
		return err
	}
	return s.uow.Do(ctx, func(txCtx context.Context) error {
		if err := s.quoteRepo.AppendHistory(txCtx, domain.QuoteHistory{
			Pair:     q.Pair,
			Price:    q.Price,
			QuotedAt: q.UpdatedAt,
			Source:   source,
			UpdateID: &updateID,
		}); err != nil {
			return err
		}
		if err := s.quoteRepo.Upsert(txCtx, domain.Quote{
			Pair:      q.Pair,
			Price:     q.Price,
			UpdatedAt: q.UpdatedAt,
		}); err != nil {
			return err
		}
		return s.updateJobRepo.UpdateStatus(txCtx, updateID, domain.QuoteUpdateStatusDone, nil)
	})
}

// ProcessQueueBatch claims queued jobs and processes them using the service's RateProvider.
// Best-effort: errors are aggregated and returned as a single error if any occurred.
func (s *FXRatesService) ProcessQueueBatch(
	ctx context.Context,
	batchLimit int,
) error {
	jobs, err := s.updateJobRepo.ClaimQueued(ctx, batchLimit)
	if err != nil {
		return err
	}
	var firstErr error
	for _, j := range jobs {
		_ = s.updateJobRepo.UpdateStatus(ctx, j.ID, domain.QuoteUpdateStatusProcessing, nil)
		err := s.CompleteQuoteUpdate(ctx, j.ID, func(c context.Context) (domain.Quote, error) {
			return s.FetchQuote(c, j.Pair)
		}, "db")
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
