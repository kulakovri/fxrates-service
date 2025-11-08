package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
)

const (
	exchangeRatesLatestPath = "/v1/latest"
)

type ExchangeRatesAPIProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

var _ application.RateProvider = (*ExchangeRatesAPIProvider)(nil)

type xrLatestResp struct {
	Success   bool               `json:"success"`
	Timestamp int64              `json:"timestamp"`
	Base      string             `json:"base"`
	Date      string             `json:"date"`
	Rates     map[string]float64 `json:"rates"`
	Error     *struct {
		Code int    `json:"code"`
		Info string `json:"info"`
	} `json:"error,omitempty"`
}

func (p *ExchangeRatesAPIProvider) Get(ctx context.Context, pair string) (domain.Quote, error) {
	if p.BaseURL == "" || p.APIKey == "" {
		return domain.Quote{}, errors.New("exchangeratesapi: missing configuration")
	}

	if !domain.IsSupportedPair(domain.Pair(pair)) {
		return domain.Quote{}, fmt.Errorf("unsupported pair: %s", pair)
	}

	baseCur, quoteCur, ok := domain.SplitPair(pair)
	if !ok {
		return domain.Quote{}, fmt.Errorf("invalid pair format: %s", pair)
	}

	u, err := url.Parse(p.BaseURL)
	if err != nil {
		return domain.Quote{}, fmt.Errorf("exchangeratesapi: invalid base url: %w", err)
	}
	u.Path = exchangeRatesLatestPath
	q := u.Query()
	q.Set("access_key", p.APIKey)
	q.Set("symbols", "USD,EUR,MXN")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return domain.Quote{}, fmt.Errorf("exchangeratesapi: create request: %w", err)
	}

	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return domain.Quote{}, fmt.Errorf("exchangeratesapi: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.Quote{}, fmt.Errorf("exchangeratesapi: status %d", resp.StatusCode)
	}

	var body xrLatestResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return domain.Quote{}, fmt.Errorf("exchangeratesapi: decode response: %w", err)
	}
	if !body.Success {
		if body.Error != nil {
			return domain.Quote{}, fmt.Errorf("exchangeratesapi: %d %s", body.Error.Code, body.Error.Info)
		}
		return domain.Quote{}, errors.New("exchangeratesapi: unsuccessful response")
	}

	eurTo := func(c string) (float64, error) {
		if c == body.Base {
			return 1.0, nil
		}
		v, ok := body.Rates[c]
		if !ok {
			return 0, fmt.Errorf("exchangeratesapi: missing rate for %s", c)
		}
		return v, nil
	}

	eurToBase, err := eurTo(baseCur)
	if err != nil {
		return domain.Quote{}, err
	}
	eurToQuote, err := eurTo(quoteCur)
	if err != nil {
		return domain.Quote{}, err
	}

	var price float64
	switch {
	case baseCur == body.Base:
		price = eurToQuote
	case quoteCur == body.Base:
		if eurToBase == 0 {
			return domain.Quote{}, errors.New("exchangeratesapi: zero rate for base currency")
		}
		price = 1.0 / eurToBase
	default:
		if eurToBase == 0 {
			return domain.Quote{}, errors.New("exchangeratesapi: zero rate for base currency")
		}
		price = eurToQuote / eurToBase
	}

	updatedAt := time.Now().UTC()
	if body.Timestamp > 0 {
		updatedAt = time.Unix(body.Timestamp, 0).UTC()
	}

	return domain.Quote{
		Pair:      domain.Pair(pair),
		Price:     price,
		UpdatedAt: updatedAt,
	}, nil
}
