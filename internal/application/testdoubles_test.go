package application

import (
	"context"
	"errors"

	"fxrates-service/internal/domain"
)

var (
	ErrRepo = errors.New("repo error")
)

type fakeQuoteRepo struct {
	store map[string]domain.Quote
	err   error
}

func (f *fakeQuoteRepo) GetLast(_ context.Context, pair string) (domain.Quote, error) {
	if f.err != nil {
		return domain.Quote{}, f.err
	}
	q, ok := f.store[pair]
	if !ok {
		return domain.Quote{}, ErrNotFound
	}
	return q, nil
}

func (f *fakeQuoteRepo) Upsert(_ context.Context, q domain.Quote) error {
	if f.err != nil {
		return f.err
	}
	if f.store == nil {
		f.store = map[string]domain.Quote{}
	}
	f.store[string(q.Pair)] = q
	return nil
}

type fakeUpdateJobRepo struct {
	jobs map[string]domain.QuoteUpdate
	err  error
}

func (f *fakeUpdateJobRepo) CreateQueued(_ context.Context, pair string, idem *string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.jobs == nil {
		f.jobs = map[string]domain.QuoteUpdate{}
	}
	id := "update-1"
	f.jobs[id] = domain.QuoteUpdate{ID: id, Pair: domain.Pair(pair), Status: domain.QuoteUpdateStatusQueued}
	return id, nil
}

func (f *fakeUpdateJobRepo) GetByID(_ context.Context, id string) (domain.QuoteUpdate, error) {
	if f.err != nil {
		return domain.QuoteUpdate{}, f.err
	}
	j, ok := f.jobs[id]
	if !ok {
		return domain.QuoteUpdate{}, ErrNotFound
	}
	return j, nil
}

func (f *fakeUpdateJobRepo) UpdateStatus(_ context.Context, id string, st domain.QuoteUpdateStatus, errMsg *string) error {
	if f.err != nil {
		return f.err
	}
	j, ok := f.jobs[id]
	if !ok {
		return ErrNotFound
	}
	j.Status, j.Error = st, errMsg
	f.jobs[id] = j
	return nil
}

type fakeRateProvider struct {
	out domain.Quote
	err error
}

func (f *fakeRateProvider) Get(context.Context, string) (domain.Quote, error) {
	if f.err != nil {
		return domain.Quote{}, f.err
	}
	return f.out, nil
}
