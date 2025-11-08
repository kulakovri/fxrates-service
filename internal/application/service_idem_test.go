package application

import (
	"context"
	"fxrates-service/internal/domain"
	"testing"
)

type fakeIdem struct{ seen map[string]bool }

func (f *fakeIdem) TryReserve(_ context.Context, k string) (bool, error) {
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[k] {
		return false, nil
	}
	f.seen[k] = true
	return true, nil
}

func TestRequestQuoteUpdate_Idempotency_Conflict(treated *testing.T) {
	idem := &fakeIdem{}
	svc := NewFXRatesService(nil, &fakeUpdateJobRepo{jobs: map[string]domain.QuoteUpdate{}}, nil, idem)
	key := "ik-1"
	if _, err := svc.RequestQuoteUpdate(context.Background(), "EUR/USD", &key); err != nil {
		treated.Fatalf("unexpected err first call: %v", err)
	}
	if _, err := svc.RequestQuoteUpdate(context.Background(), "EUR/USD", &key); err == nil {
		treated.Fatalf("expected conflict on duplicate key")
	}
}
