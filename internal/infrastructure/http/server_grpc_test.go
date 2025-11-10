package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fxrates-service/internal/config"
	"fxrates-service/internal/infrastructure/grpc/ratepb"

	"github.com/stretchr/testify/require"
)

type fakeRateClient struct {
	pair   string
	price  float64
	nowStr string
}

func (f *fakeRateClient) Fetch(ctx context.Context, pair, traceID string, timeout time.Duration) (*ratepb.FetchResponse, error) {
	return &ratepb.FetchResponse{
		Pair:      pair,
		Price:     f.price,
		UpdatedAt: f.nowStr,
	}, nil
}

func TestRequestQuoteUpdate_AsyncGRPCBackground(t *testing.T) {
	// Prepare service and repos
	svc, qr, ur, _ := NewInMemoryService()
	srv := NewServer(svc)
	// Attach fake gRPC client and repos with grpc mode config
	now := time.Now().UTC().Format(time.RFC3339Nano)
	srv.AttachGRPCBackground(qr, ur, &fakeRateClient{price: 2.5, nowStr: now}, config.Config{
		WorkerType:     "grpc",
		RequestTimeout: 2 * time.Second,
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
