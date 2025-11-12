package httpserver

import (
	"context"
	"errors"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"sync"
)

var _ application.QuoteRepo = (*fakeQuoteRepo)(nil)
var _ application.UpdateJobRepo = (*fakeUpdateJobRepo)(nil)
var _ application.RateProvider = (*fakeRateProvider)(nil)

type fakeQuoteRepo struct {
	mu    sync.RWMutex
	store map[string]domain.Quote
}

func (f *fakeQuoteRepo) GetLast(_ context.Context, pair string) (domain.Quote, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.store == nil {
		return domain.Quote{}, domain.ErrNotFound
	}
	q, ok := f.store[pair]
	if !ok {
		return domain.Quote{}, domain.ErrNotFound
	}
	return q, nil
}

func (f *fakeQuoteRepo) Upsert(_ context.Context, q domain.Quote) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.store == nil {
		f.store = map[string]domain.Quote{}
	}
	f.store[string(q.Pair)] = q
	return nil
}

func (f *fakeQuoteRepo) AppendHistory(_ context.Context, _ domain.QuoteHistory) error {
	return nil
}

type fakeUpdateJobRepo struct {
	mu   sync.RWMutex
	jobs map[string]domain.QuoteUpdate
}

func (f *fakeUpdateJobRepo) CreateQueued(_ context.Context, pair string, _ *string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.jobs == nil {
		f.jobs = map[string]domain.QuoteUpdate{}
	}
	id := "update-1"
	f.jobs[id] = domain.QuoteUpdate{ID: id, Pair: domain.Pair(pair), Status: domain.QuoteUpdateStatusQueued, UpdatedAt: time.Now()}
	return id, nil
}

func (f *fakeUpdateJobRepo) GetByID(_ context.Context, id string) (domain.QuoteUpdate, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.jobs == nil {
		return domain.QuoteUpdate{}, domain.ErrNotFound
	}
	j, ok := f.jobs[id]
	if !ok {
		return domain.QuoteUpdate{}, domain.ErrNotFound
	}
	return j, nil
}

func (f *fakeUpdateJobRepo) UpdateStatus(_ context.Context, id string, st domain.QuoteUpdateStatus, errMsg *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.jobs == nil {
		return errors.New("no jobs")
	}
	j, ok := f.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	j.Status = st
	j.Error = errMsg
	j.UpdatedAt = time.Now()
	f.jobs[id] = j
	return nil
}

func (f *fakeUpdateJobRepo) ListQueuedIDs() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var ids []string
	for id, j := range f.jobs {
		if j.Status == domain.QuoteUpdateStatusQueued {
			ids = append(ids, id)
		}
	}
	return ids
}

func (f *fakeUpdateJobRepo) ClaimQueued(_ context.Context, limit int) ([]struct{ ID, Pair string }, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []struct{ ID, Pair string }
	for id, j := range f.jobs {
		if j.Status == domain.QuoteUpdateStatusQueued {
			// claim
			j.Status = domain.QuoteUpdateStatusProcessing
			f.jobs[id] = j
			out = append(out, struct{ ID, Pair string }{ID: id, Pair: string(j.Pair)})
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

type fakeRateProvider struct{}

func (fakeRateProvider) Get(_ context.Context, pair string) (domain.Quote, error) {
	return domain.Quote{Pair: domain.Pair(pair), Price: 0, UpdatedAt: time.Now()}, nil
}

func NewInMemoryService() (*application.FXRatesService, *fakeQuoteRepo, *fakeUpdateJobRepo, fakeRateProvider) {
	qr := &fakeQuoteRepo{store: map[string]domain.Quote{}}
	ur := &fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}}
	rp := fakeRateProvider{}
	return application.NewService(qr, ur, rp, application.NoopIdempotency{}), qr, ur, rp
}

func NewInMemoryRepos() (application.QuoteRepo, application.UpdateJobRepo, application.RateProvider) {
	qr := &fakeQuoteRepo{store: map[string]domain.Quote{}}
	ur := &fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}}
	rp := fakeRateProvider{}
	return qr, ur, rp
}

func NewFakeRateProvider() application.RateProvider { return fakeRateProvider{} }
