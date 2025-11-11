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
	u, _ := url.Parse(p.BaseURL)
	u.Path = exchangeRatesLatestPath
	q := u.Query()
	q.Set("access_key", p.APIKey)
	q.Set("symbols", pair)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)

	var res apiResponse
	if err := p.Client.DoJSON(ctx, req, &res); err != nil {
		return domain.Quote{}, fmt.Errorf("provider: %w", err)
	}
	return domain.Quote{
		Pair:      domain.Pair(pair),
		Price:     res.Rates[pair],
		UpdatedAt: time.Unix(res.Timestamp, 0).UTC(),
	}, nil
}
