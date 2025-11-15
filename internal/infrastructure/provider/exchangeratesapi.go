package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/httpx"
)

const (
	exchangeRatesLatestPath = "/v1/latest"
)

type ExchangeRatesAPIProvider struct {
	BaseURL string
	APIKey  string
	Client  *httpx.Client
	// Optional backoff config; if nil, httpx defaults apply. Prefer wiring from config.
	BackoffCfg *httpx.BackoffConfig
}

var _ application.RateProvider = (*ExchangeRatesAPIProvider)(nil)

type apiResponse struct {
	Success   bool               `json:"success"`
	Timestamp int64              `json:"timestamp"`
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
	Error     *struct {
		Code int    `json:"code"`
		Info string `json:"info"`
	} `json:"error,omitempty"`
}

func (p *ExchangeRatesAPIProvider) Get(ctx context.Context, pair string) (domain.Quote, error) {
	if !domain.ValidatePair(pair) {
		return domain.Quote{}, fmt.Errorf("provider: invalid pair %q", pair)
	}
	base := pair[:3]
	quote := pair[4:]

	u, _ := url.Parse(p.BaseURL)
	u.Path = exchangeRatesLatestPath
	q := u.Query()
	q.Set("access_key", p.APIKey)
	// Avoid base param (restricted on free plans). Request both currencies and compute cross-rate.
	q.Set("symbols", base+","+quote)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)

	var res apiResponse
	if err := p.Client.DoJSON(ctx, req, &res, p.BackoffCfg); err != nil {
		return domain.Quote{}, fmt.Errorf("provider: %w", err)
	}

	if !res.Success || res.Error != nil {
		if res.Error != nil {
			return domain.Quote{}, fmt.Errorf("provider: api_error code=%d info=%s", res.Error.Code, res.Error.Info)
		}
		return domain.Quote{}, fmt.Errorf("provider: api_error")
	}
	// Prefer exact pair key if present (supports tests or providers that return "EUR/USD")
	rate, ok := res.Rates[pair]
	if !ok {
		// Compute cross-rate using provider base (typically EUR on free tier).
		switch {
		case res.Base == base:
			// Pair base equals provider base: direct quote
			r, ok := res.Rates[quote]
			if !ok {
				return domain.Quote{}, fmt.Errorf("provider: missing rate for %s", quote)
			}
			rate = r
		case res.Base == quote:
			// Pair quote equals provider base: invert base
			bv, ok := res.Rates[base]
			if !ok || bv == 0 {
				return domain.Quote{}, fmt.Errorf("provider: missing or zero rate for %s", base)
			}
			rate = 1 / bv
		default:
			// Cross: QUOTE_per_BASE = (QUOTE_per_RESBASE) / (BASE_per_RESBASE)
			qv, ok1 := res.Rates[quote]
			bv, ok2 := res.Rates[base]
			if !ok1 || !ok2 || bv == 0 {
				return domain.Quote{}, fmt.Errorf("provider: missing rates for %s or %s", base, quote)
			}
			rate = qv / bv
		}
	}
	return domain.Quote{
		Pair:      domain.Pair(pair),
		Price:     rate,
		UpdatedAt: time.Unix(res.Timestamp, 0).UTC(),
	}, nil
}
