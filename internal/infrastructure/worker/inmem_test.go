package worker

import (
	"context"
	"testing"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"github.com/stretchr/testify/require"
	"sync"
)

type memQuotes struct {
	mu    sync.RWMutex
	store map[string]domain.Quote
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
func (m *memQuotes) has(pair string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.store == nil {
		return false
	}
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

// implement ListQueuedIDs so worker can discover jobs
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
