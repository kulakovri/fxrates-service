package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":400,"message":"invalid pair format (e.g. EUR/USD)"}`, rec.Body.String())
}
