package httpserver

import (
	"context"
	"errors"
	"fxrates-service/internal/domain"
	openapi "fxrates-service/internal/infrastructure/http/openapi"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_mapStatus(t *testing.T) {
	cases := []struct {
		in  domain.QuoteUpdateStatus
		out openapi.QuoteUpdateDetailsStatus
	}{
		{domain.QuoteUpdateStatusQueued, openapi.Pending},
		{domain.QuoteUpdateStatusProcessing, openapi.Pending},
		{domain.QuoteUpdateStatusDone, openapi.Completed},
		{domain.QuoteUpdateStatusFailed, openapi.Failed},
	}
	for _, c := range cases {
		got := mapStatus(c.in)
		if got != c.out {
			t.Fatalf("mapStatus(%v)=%v want %v", c.in, got, c.out)
		}
	}
}

func Test_readyz_FailingCheck(t *testing.T) {
	svc, _, _, _ := NewInMemoryService()
	srv := NewServer(svc)
	srv.SetReadyCheck(func(ctx context.Context) error { return errors.New("db down") })
	h := NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	want := `{"code":503,"message":"db not ready"}`
	// Compare JSON structurally to ignore whitespace/newlines
	if rec.Body.Len() == 0 {
		t.Fatalf("empty body")
	}
	// Use JSONEq to avoid formatting differences
	require.JSONEq(t, want, rec.Body.String())
}
