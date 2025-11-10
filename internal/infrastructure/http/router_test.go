package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"

	"github.com/stretchr/testify/require"
)

func setup() http.Handler {
	svc, _, _, _ := NewInMemoryService()
	srv := NewServer(svc)
	return NewRouter(srv)
}

func TestHealthz(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "OK", rec.Body.String())
}

func TestReadyz(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "READY", rec.Body.String())
}

func TestRequestQuoteUpdate(t *testing.T) {
	h := setup()
	body := map[string]string{"pair": "EUR/USD"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/quotes/updates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "k1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)
	var resp struct {
		UpdateID string `json:"update_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.Equal(t, "update-1", resp.UpdateID)
}

func TestGetQuoteUpdate_NotFound(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/quotes/updates/nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"code":404,"message":"not found"}`, rec.Body.String())
}

func TestGetLastQuote_EmptyStore(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/quotes/last?pair=EUR/USD", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"code":404,"message":"not found"}`, rec.Body.String())
}

func TestRequestQuoteUpdate_InvalidPair(t *testing.T) {
	h := setup()
	body := map[string]string{"pair": "eur/usd"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/quotes/updates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "k-bad-format")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":400,"message":"invalid pair format (e.g. EUR/USD)"}`, rec.Body.String())
}

func TestRequestQuoteUpdate_UnsupportedPair_HTTP(t *testing.T) {
	h := setup()
	body := map[string]string{"pair": "GBP/USD"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/quotes/updates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "k2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":400,"message":"unsupported pair"}`, rec.Body.String())
}

func TestGetLastQuote_UnsupportedPair_HTTP(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/quotes/last?pair=GBP/USD", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":400,"message":"unsupported pair"}`, rec.Body.String())
}
func TestGetQuoteUpdate_WithPrice(t *testing.T) {
	// Prepare in-memory service and pre-populate a completed update with price and timestamp
	svc, _, ur, _ := NewInMemoryService()
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	price := 1.234567
	ur.mu.Lock()
	ur.jobs["update-1"] = domain.QuoteUpdate{
		ID:        "update-1",
		Pair:      "EUR/USD",
		Status:    domain.QuoteUpdateStatusDone,
		Price:     &price,
		UpdatedAt: ts,
	}
	ur.mu.Unlock()

	srv := NewServer(svc)
	h := NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/quotes/updates/update-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		UpdateID  string    `json:"update_id"`
		Pair      string    `json:"pair"`
		Status    string    `json:"status"`
		Price     *float32  `json:"price"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "update-1", resp.UpdateID)
	require.Equal(t, "EUR/USD", resp.Pair)
	require.Equal(t, "done", resp.Status)
	require.NotNil(t, resp.Price)
	require.InDelta(t, float64(*resp.Price), price, 1e-5)
	require.Equal(t, ts, resp.UpdatedAt)
}

type memIdem struct{ seen map[string]bool }

func (m *memIdem) TryReserve(_ context.Context, k string) (bool, error) {
	if m.seen == nil {
		m.seen = map[string]bool{}
	}
	if m.seen[k] {
		return false, nil
	}
	m.seen[k] = true
	return true, nil
}

func TestRequestQuoteUpdate_IdempotencyConflict_HTTP(t *testing.T) {
	qr, ur, rp := NewInMemoryRepos()
	idem := &memIdem{}
	svc := application.NewFXRatesService(qr, ur, rp, idem)
	srv := NewServer(svc)
	h := NewRouter(srv)

	body := map[string]string{"pair": "EUR/USD"}
	b, _ := json.Marshal(body)
	// First call should 202
	req1 := httptest.NewRequest(http.MethodPost, "/quotes/updates", bytes.NewReader(b))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Idempotency-Key", "k-dup")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusAccepted, rec1.Code)

	// Second call should 409 conflict with JSON Error envelope
	req2 := httptest.NewRequest(http.MethodPost, "/quotes/updates", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Idempotency-Key", "k-dup")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusConflict, rec2.Code)
	require.JSONEq(t, `{"code":409,"message":"conflict"}`, rec2.Body.String())
}
