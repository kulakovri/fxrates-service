package httpserver

import (
	"context"
	"errors"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
)

var _ application.QuoteRepo = (*fakeQuoteRepo)(nil)
var _ application.UpdateJobRepo = (*fakeUpdateJobRepo)(nil)
var _ application.RateProvider = (*fakeRateProvider)(nil)

type fakeQuoteRepo struct {
	store map[string]domain.Quote
}

func (f *fakeQuoteRepo) GetLast(_ context.Context, pair string) (domain.Quote, error) {
	if f.store == nil {
		return domain.Quote{}, ErrNotFound
	}
	q, ok := f.store[pair]
	if !ok {
		return domain.Quote{}, ErrNotFound
	}
	return q, nil
}

func (f *fakeQuoteRepo) Upsert(_ context.Context, q domain.Quote) error {
	if f.store == nil {
		f.store = map[string]domain.Quote{}
	}
	f.store[string(q.Pair)] = q
	return nil
}

type fakeUpdateJobRepo struct {
	jobs map[string]domain.QuoteUpdate
}

func (f *fakeUpdateJobRepo) CreateQueued(_ context.Context, pair string, _ *string) (string, error) {
	if f.jobs == nil {
		f.jobs = map[string]domain.QuoteUpdate{}
	}
	id := "update-1"
	f.jobs[id] = domain.QuoteUpdate{ID: id, Pair: domain.Pair(pair), Status: domain.QuoteUpdateStatusQueued, UpdatedAt: time.Now()}
	return id, nil
}

func (f *fakeUpdateJobRepo) GetByID(_ context.Context, id string) (domain.QuoteUpdate, error) {
	if f.jobs == nil {
		return domain.QuoteUpdate{}, ErrNotFound
	}
	j, ok := f.jobs[id]
	if !ok {
		return domain.QuoteUpdate{}, ErrNotFound
	}
	return j, nil
}

func (f *fakeUpdateJobRepo) UpdateStatus(_ context.Context, id string, st domain.QuoteUpdateStatus, errMsg *string) error {
	if f.jobs == nil {
		return errors.New("no jobs")
	}
	j, ok := f.jobs[id]
	if !ok {
		return ErrNotFound
	}
	j.Status = st
	j.Error = errMsg
	j.UpdatedAt = time.Now()
	f.jobs[id] = j
	return nil
}

func (f *fakeUpdateJobRepo) ListQueuedIDs() []string {
	var ids []string
	for id, j := range f.jobs {
		if j.Status == domain.QuoteUpdateStatusQueued {
			ids = append(ids, id)
		}
	}
	return ids
}

type fakeRateProvider struct{}

func (fakeRateProvider) Get(_ context.Context, pair string) (domain.Quote, error) {
	return domain.Quote{Pair: domain.Pair(pair), Price: 0, UpdatedAt: time.Now()}, nil
}

func NewInMemoryService() (*application.FXRatesService, *fakeQuoteRepo, *fakeUpdateJobRepo, fakeRateProvider) {
	qr := &fakeQuoteRepo{store: map[string]domain.Quote{}}
	ur := &fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}}
	rp := fakeRateProvider{}
	return application.NewFXRatesService(qr, ur, rp), qr, ur, rp
}
