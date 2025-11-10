package worker

// test-only implementation of an in-memory worker used by unit tests.
// It is not compiled into production binaries.

import (
	"context"
	"sync"
	"testing"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"github.com/stretchr/testify/require"
)

type InMemWorker struct {
	Updates   application.UpdateJobRepo
	Quotes    application.QuoteRepo
	Provider  application.RateProvider
	PollEvery time.Duration
}

type queuedLister interface{ ListQueuedIDs() []string }

func queuedIDs(repo application.UpdateJobRepo) []string {
	if l, ok := repo.(queuedLister); ok {
		return l.ListQueuedIDs()
	}
	return nil
}

func (w *InMemWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.PollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids := queuedIDs(w.Updates)
			for _, id := range ids {
				w.processJob(ctx, id)
			}
		}
	}
}

func (w *InMemWorker) processJob(ctx context.Context, id string) {
	job, err := w.Updates.GetByID(ctx, id)
	if err != nil {
		return
	}

	_ = w.Updates.UpdateStatus(ctx, id, domain.QuoteUpdateStatusProcessing, nil)

	q, err := w.Provider.Get(ctx, string(job.Pair))
	if err != nil {
		msg := err.Error()
		_ = w.Updates.UpdateStatus(ctx, id, domain.QuoteUpdateStatusFailed, &msg)
		return
	}

	_ = w.Quotes.Upsert(ctx, q)
	_ = w.Updates.UpdateStatus(ctx, id, domain.QuoteUpdateStatusDone, nil)
}

// ----- Test fakes and test -----

type memQuotes struct {
	mu      sync.RWMutex
	store   map[string]domain.Quote
	history []domain.QuoteHistory
}

func (m *memQuotes) GetLast(context.Context, string) (domain.Quote, error) {
	return domain.Quote{}, nil
}
func (m *memQuotes) Upsert(_ context.Context, q domain.Quote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store == nil {
		m.store = map[string]domain.Quote{}
	}
	m.store[string(q.Pair)] = q
	return nil
}
func (m *memQuotes) AppendHistory(_ context.Context, h domain.QuoteHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, h)
	return nil
}
func (m *memQuotes) has(pair string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[pair]
	return ok
}

type memJobs struct {
	mu   sync.RWMutex
	jobs map[string]domain.QuoteUpdate
}

func (m *memJobs) CreateQueued(context.Context, string, *string) (string, error) { return "", nil }
func (m *memJobs) GetByID(_ context.Context, id string) (domain.QuoteUpdate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[id], nil
}
func (m *memJobs) UpdateStatus(_ context.Context, id string, st domain.QuoteUpdateStatus, errMsg *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[id]
	j.Status, j.Error = st, errMsg
	m.jobs[id] = j
	return nil
}
func (m *memJobs) ListQueuedIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for id, j := range m.jobs {
		if j.Status == domain.QuoteUpdateStatusQueued {
			ids = append(ids, id)
		}
	}
	return ids
}
func (m *memJobs) ClaimQueued(_ context.Context, limit int) ([]struct{ ID, Pair string }, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []struct{ ID, Pair string }
	for id, j := range m.jobs {
		if j.Status == domain.QuoteUpdateStatusQueued {
			j.Status = domain.QuoteUpdateStatusProcessing
			m.jobs[id] = j
			out = append(out, struct{ ID, Pair string }{ID: id, Pair: string(j.Pair)})
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
func (m *memJobs) status(id string) domain.QuoteUpdateStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[id].Status
}

type memProvider struct{ price float64 }

func (p *memProvider) Get(context.Context, string) (domain.Quote, error) {
	return domain.Quote{Pair: "EUR/USD", Price: p.price, UpdatedAt: time.Now()}, nil
}

func TestInMemWorker_ProcessJob(t *testing.T) {
	j := &memJobs{jobs: map[string]domain.QuoteUpdate{
		"update-1": {ID: "update-1", Pair: "EUR/USD", Status: domain.QuoteUpdateStatusQueued},
	}}
	q := &memQuotes{}
	p := &memProvider{price: 1.23}

	var _ application.UpdateJobRepo = j
	var _ application.QuoteRepo = q

	w := &InMemWorker{Updates: j, Quotes: q, Provider: p, PollEvery: 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	go w.Start(ctx)
	time.Sleep(30 * time.Millisecond)

	require.Equal(t, domain.QuoteUpdateStatusDone, j.status("update-1"))
	require.True(t, q.has("EUR/USD"))
}
