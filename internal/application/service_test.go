package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"fxrates-service/internal/domain"
)

type fakeQuoteRepo struct {
	quotes map[string]domain.Quote
	err    error
}

func (f *fakeQuoteRepo) GetLast(ctx context.Context, pair string) (domain.Quote, error) {
	if f.err != nil {
		return domain.Quote{}, f.err
	}
	if quote, ok := f.quotes[pair]; ok {
		return quote, nil
	}
	return domain.Quote{}, errors.New("quote not found")
}

func (f *fakeQuoteRepo) Upsert(ctx context.Context, q domain.Quote) error {
	if f.err != nil {
		return f.err
	}
	if f.quotes == nil {
		f.quotes = make(map[string]domain.Quote)
	}
	f.quotes[string(q.Pair)] = q
	return nil
}

type fakeUpdateJobRepo struct {
	jobs map[string]domain.QuoteUpdate
	err  error
	id   int
}

func (f *fakeUpdateJobRepo) CreateQueued(ctx context.Context, pair string, idem *string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.jobs == nil {
		f.jobs = make(map[string]domain.QuoteUpdate)
	}
	f.id++
	updateID := fmt.Sprintf("update-%d", f.id)
	f.jobs[updateID] = domain.QuoteUpdate{
		ID:        updateID,
		Pair:      domain.Pair(pair),
		Status:    domain.QuoteUpdateStatusQueued,
		UpdatedAt: time.Now(),
	}
	return updateID, nil
}

func (f *fakeUpdateJobRepo) GetByID(ctx context.Context, id string) (domain.QuoteUpdate, error) {
	if f.err != nil {
		return domain.QuoteUpdate{}, f.err
	}
	if job, ok := f.jobs[id]; ok {
		return job, nil
	}
	return domain.QuoteUpdate{}, errors.New("update not found")
}

func (f *fakeUpdateJobRepo) UpdateStatus(ctx context.Context, id string, status domain.QuoteUpdateStatus, errMsg *string) error {
	if f.err != nil {
		return f.err
	}
	if job, ok := f.jobs[id]; ok {
		job.Status = status
		job.Error = errMsg
		f.jobs[id] = job
		return nil
	}
	return errors.New("update not found")
}

type fakeRateProvider struct {
	quotes map[string]domain.Quote
	err    error
}

func (f *fakeRateProvider) Get(ctx context.Context, pair string) (domain.Quote, error) {
	if f.err != nil {
		return domain.Quote{}, f.err
	}
	if quote, ok := f.quotes[pair]; ok {
		return quote, nil
	}
	return domain.Quote{}, errors.New("rate not found")
}

func TestFXRatesService_RequestQuoteUpdate(t *testing.T) {
	tests := []struct {
		name      string
		pair      string
		idem      *string
		repoErr   error
		wantErr   bool
		wantIDLen int
	}{
		{
			name:      "successful request",
			pair:      "EUR/USD",
			idem:      stringPtr("key-123"),
			wantErr:   false,
			wantIDLen: 8, // "update-1" = 8 chars
		},
		{
			name:      "successful request without idempotency key",
			pair:      "GBP/USD",
			idem:      nil,
			wantErr:   false,
			wantIDLen: 8,
		},
		{
			name:    "repository error",
			pair:    "EUR/USD",
			idem:    nil,
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUpdateJobRepo{
				jobs: make(map[string]domain.QuoteUpdate),
				err:  tt.repoErr,
			}
			service := NewFXRatesService(nil, repo, nil)

			updateID, err := service.RequestQuoteUpdate(context.Background(), tt.pair, tt.idem)

			if (err != nil) != tt.wantErr {
				t.Errorf("RequestQuoteUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(updateID) < tt.wantIDLen {
				t.Errorf("RequestQuoteUpdate() updateID = %v, want length >= %d", updateID, tt.wantIDLen)
			}
		})
	}
}

func TestFXRatesService_GetQuoteUpdate(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		setup   func(*fakeUpdateJobRepo)
		wantErr bool
		wantID  string
	}{
		{
			name: "successful retrieval",
			id:   "update-1",
			setup: func(repo *fakeUpdateJobRepo) {
				repo.jobs = map[string]domain.QuoteUpdate{
					"update-1": {
						ID:        "update-1",
						Pair:      "EUR/USD",
						Status:    domain.QuoteUpdateStatusQueued,
						UpdatedAt: time.Now(),
					},
				}
			},
			wantErr: false,
			wantID:  "update-1",
		},
		{
			name: "not found",
			id:   "update-999",
			setup: func(repo *fakeUpdateJobRepo) {
				repo.jobs = make(map[string]domain.QuoteUpdate)
			},
			wantErr: true,
		},
		{
			name: "repository error",
			id:   "update-1",
			setup: func(repo *fakeUpdateJobRepo) {
				repo.err = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUpdateJobRepo{
				jobs: make(map[string]domain.QuoteUpdate),
			}
			if tt.setup != nil {
				tt.setup(repo)
			}
			service := NewFXRatesService(nil, repo, nil)

			update, err := service.GetQuoteUpdate(context.Background(), tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetQuoteUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && update.ID != tt.wantID {
				t.Errorf("GetQuoteUpdate() update.ID = %v, want %v", update.ID, tt.wantID)
			}
		})
	}
}

func TestFXRatesService_GetLastQuote(t *testing.T) {
	tests := []struct {
		name     string
		pair     string
		setup    func(*fakeQuoteRepo)
		wantErr  bool
		wantPair domain.Pair
	}{
		{
			name: "successful retrieval",
			pair: "EUR/USD",
			setup: func(repo *fakeQuoteRepo) {
				repo.quotes = map[string]domain.Quote{
					"EUR/USD": {
						Pair:      "EUR/USD",
						Price:     1.10,
						UpdatedAt: time.Now(),
					},
				}
			},
			wantErr:  false,
			wantPair: "EUR/USD",
		},
		{
			name: "not found",
			pair: "GBP/USD",
			setup: func(repo *fakeQuoteRepo) {
				repo.quotes = make(map[string]domain.Quote)
			},
			wantErr: true,
		},
		{
			name: "repository error",
			pair: "EUR/USD",
			setup: func(repo *fakeQuoteRepo) {
				repo.err = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeQuoteRepo{
				quotes: make(map[string]domain.Quote),
			}
			if tt.setup != nil {
				tt.setup(repo)
			}
			service := NewFXRatesService(repo, nil, nil)

			quote, err := service.GetLastQuote(context.Background(), tt.pair)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetLastQuote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && quote.Pair != tt.wantPair {
				t.Errorf("GetLastQuote() quote.Pair = %v, want %v", quote.Pair, tt.wantPair)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
