package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fxrates-service/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestRequestQuoteUpdate_AsyncGRPCBackground(t *testing.T) {
	// Prepare service and repos
	svc, qr, ur, _ := NewInMemoryService()
	srv := NewServer(svc)
	// Install a dispatcher that simulates gRPC background completion
	const price = 2.5
	srv.SetDispatcher(func(ctx context.Context, updateID, pair, traceID string) error {
		go func() {
			_ = svc.CompleteQuoteUpdate(context.Background(), updateID, func(context.Context) (domain.Quote, error) {
				return domain.Quote{
					Pair:      domain.Pair(pair),
					Price:     price,
					UpdatedAt: time.Now().UTC(),
				}, nil
			}, "grpc")
		}()
		return nil
	})
	h := NewRouter(srv)

	// Send request
	body := map[string]string{"pair": "EUR/USD"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/quotes/updates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "idem-1")
	req.Header.Set("X-Trace-Id", "tid-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	// Wait briefly for background goroutine
	time.Sleep(50 * time.Millisecond)

	// Verify update job status is done
	upd, err := ur.GetByID(context.Background(), "update-1")
	require.NoError(t, err)
	require.Equal(t, "done", string(upd.Status))
	// Verify quote upserted
	q, err := qr.GetLast(context.Background(), "EUR/USD")
	require.NoError(t, err)
	require.InDelta(t, 2.5, q.Price, 0.000001)
}
