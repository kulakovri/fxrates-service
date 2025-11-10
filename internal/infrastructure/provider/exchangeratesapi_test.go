package provider_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fxrates-service/internal/infrastructure/provider"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

func httpClient(resBody string, code int) *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripFunc(func(r *http.Request) *http.Response {
			return &http.Response{
				StatusCode: code,
				Body:       io.NopCloser(strings.NewReader(resBody)),
				Header:     make(http.Header),
			}
		}),
	}
}

const sampleOK = `{
  "success": true,
  "base": "EUR",
  "date": "2025-11-08",
  "rates": { "USD": 1.20, "MXN": 20.00, "EUR": 1.0 }
}`

func TestGet_USD_EUR(t *testing.T) {
	p := &provider.ExchangeRatesAPIProvider{
		BaseURL: "https://api.exchangeratesapi.io",
		APIKey:  "test",
		Client:  httpClient(sampleOK, 200),
	}
	q, err := p.Get(context.Background(), "USD/EUR")
	require.NoError(t, err)
	require.InDelta(t, 0.8333, q.Price, 0.0001)
}

func TestGet_EUR_USD(t *testing.T) {
	p := &provider.ExchangeRatesAPIProvider{
		BaseURL: "https://api.exchangeratesapi.io",
		APIKey:  "test",
		Client:  httpClient(sampleOK, 200),
	}
	q, err := p.Get(context.Background(), "EUR/USD")
	require.NoError(t, err)
	require.InDelta(t, 1.20, q.Price, 0.0001)
}

func TestGet_USD_MXN(t *testing.T) {
	p := &provider.ExchangeRatesAPIProvider{
		BaseURL: "https://api.exchangeratesapi.io",
		APIKey:  "test",
		Client:  httpClient(sampleOK, 200),
	}
	q, err := p.Get(context.Background(), "USD/MXN")
	require.NoError(t, err)
	require.InDelta(t, 16.6667, q.Price, 0.0001)
}

func TestGet_UnsupportedPair(t *testing.T) {
	p := &provider.ExchangeRatesAPIProvider{
		BaseURL: "https://api.exchangeratesapi.io",
		APIKey:  "test",
		Client:  httpClient(sampleOK, 200),
	}
	_, err := p.Get(context.Background(), "USD/GBP")
	require.Error(t, err)
}

func TestGet_APIError(t *testing.T) {
	body := `{"success": false, "error": {"code": 104, "info": "quota exceeded"}}`
	p := &provider.ExchangeRatesAPIProvider{
		BaseURL: "https://api.exchangeratesapi.io",
		APIKey:  "bad",
		Client:  httpClient(body, 200),
	}
	_, err := p.Get(context.Background(), "EUR/USD")
	require.Error(t, err)
}
