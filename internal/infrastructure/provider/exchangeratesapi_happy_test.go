package provider_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fxrates-service/internal/infrastructure/httpx"
	"fxrates-service/internal/infrastructure/provider"
	"github.com/stretchr/testify/require"
)

type rtFunc func(*http.Request) *http.Response

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

func httpClient(resBody string, code int) *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: rtFunc(func(r *http.Request) *http.Response {
			return &http.Response{
				StatusCode: code,
				Body:       io.NopCloser(strings.NewReader(resBody)),
				Header:     make(http.Header),
				Request:    r,
			}
		}),
	}
}

func TestProvider_HappyPath(t *testing.T) {
	body := `{"success": true, "timestamp": 1731240000, "base":"EUR", "rates": {"EUR/USD": 1.23}}`
	client := httpClient(body, 200)
	p := &provider.ExchangeRatesAPIProvider{
		BaseURL: "http://example.com",
		APIKey:  "test",
		Client:  &httpx.Client{HTTP: client},
	}
	q, err := p.Get(context.Background(), "EUR/USD")
	require.NoError(t, err)
	require.InDelta(t, 1.23, q.Price, 0.0001)
	require.Equal(t, time.Unix(1731240000, 0).UTC(), q.UpdatedAt)
}
