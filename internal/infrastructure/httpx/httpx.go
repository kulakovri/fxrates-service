package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type Client struct {
	HTTP  *http.Client
	Token string
}

type BackoffConfig struct {
	Initial time.Duration
	Max     time.Duration
	Total   time.Duration
}

func (c *Client) DoJSON(ctx context.Context, req *http.Request, out any, cfg *BackoffConfig) error {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}

	// default backoff config
	if cfg == nil {
		cfg = &BackoffConfig{
			Initial: 200 * time.Millisecond,
			Max:     1 * time.Second,
			Total:   3 * time.Second,
		}
	}

	exp := backoff.NewExponentialBackOff()
	exp.InitialInterval = cfg.Initial
	exp.MaxInterval = cfg.Max
	exp.MaxElapsedTime = cfg.Total

	op := func() error {
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error %d", resp.StatusCode)
		}
		if resp.StatusCode != 200 {
			return backoff.Permanent(fmt.Errorf("status %d", resp.StatusCode))
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return backoff.Permanent(fmt.Errorf("decode: %w", err))
		}
		return nil
	}
	return backoff.Retry(op, backoff.WithContext(exp, ctx))
}
