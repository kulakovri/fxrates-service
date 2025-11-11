package application

import (
	"context"
	"testing"
	"time"

	"fxrates-service/internal/domain"

	"github.com/stretchr/testify/require"
)

func Test_RequestQuoteUpdate(t *testing.T) {
	t.Parallel()
	u := &fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}}
	svc := NewFXRatesService(
		&fakeQuoteRepo{store: map[string]domain.Quote{}},
		u,
		&fakeRateProvider{},
		NoopIdempotency{},
		WithClock(func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }),
		WithIDGen(func() string { return "update-1" }),
	)

	id, err := svc.RequestQuoteUpdate(context.Background(), "EUR/USD", strPtr("idem-1"))
	require.NoError(t, err)
	require.Equal(t, "update-1", id)
	require.Contains(t, u.jobs, "update-1")
	require.Equal(t, domain.QuoteUpdateStatusQueued, u.jobs["update-1"].Status)
}

func Test_RequestQuoteUpdate_UnsupportedPair(t *testing.T) {
	t.Parallel()
	svc := NewFXRatesService(
		&fakeQuoteRepo{store: map[string]domain.Quote{}},
		&fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}},
		&fakeRateProvider{},
		NoopIdempotency{},
	)

	id, err := svc.RequestQuoteUpdate(context.Background(), "GBP/USD", strPtr("idem-1"))
	require.NoError(t, err)
	require.NotEmpty(t, id)
}

func Test_GetQuoteUpdate_Found(t *testing.T) {
	t.Parallel()
	u := &fakeUpdateJobRepo{
		jobs: map[string]domain.QuoteUpdate{
			"update-1": {ID: "update-1", Pair: "EUR/USD", Status: domain.QuoteUpdateStatusQueued},
		},
	}
	svc := NewFXRatesService(&fakeQuoteRepo{}, u, &fakeRateProvider{}, NoopIdempotency{})

	got, err := svc.GetQuoteUpdate(context.Background(), "update-1")
	require.NoError(t, err)
	require.Equal(t, "update-1", got.ID)
	require.Equal(t, domain.QuoteUpdateStatusQueued, got.Status)
}

func Test_GetQuoteUpdate_NotFound(t *testing.T) {
	t.Parallel()
	u := &fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}}
	svc := NewFXRatesService(&fakeQuoteRepo{}, u, &fakeRateProvider{}, NoopIdempotency{})

	_, err := svc.GetQuoteUpdate(context.Background(), "nope")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
}

func Test_GetLastQuote(t *testing.T) {
	t.Parallel()
	qr := &fakeQuoteRepo{
		store: map[string]domain.Quote{
			"EUR/USD": {Pair: "EUR/USD", Price: 1.1, UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	svc := NewFXRatesService(qr, &fakeUpdateJobRepo{}, &fakeRateProvider{}, NoopIdempotency{})

	q, err := svc.GetLastQuote(context.Background(), "EUR/USD")
	require.NoError(t, err)
	require.Equal(t, domain.Pair("EUR/USD"), q.Pair)
	require.InDelta(t, 1.1, q.Price, 1e-9)
}

func Test_GetLastQuote_UnsupportedPair(t *testing.T) {
	t.Parallel()
	svc := NewFXRatesService(&fakeQuoteRepo{}, &fakeUpdateJobRepo{}, &fakeRateProvider{}, NoopIdempotency{})

	_, err := svc.GetLastQuote(context.Background(), "GBP/USD")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
}

func strPtr(s string) *string { return &s }
