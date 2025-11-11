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

func (c *Client) DoJSON(ctx context.Context, req *http.Request, out any) error {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}

	exp := backoff.NewExponentialBackOff()
	exp.InitialInterval = 200 * time.Millisecond
	exp.MaxInterval = 1 * time.Second
	exp.MaxElapsedTime = 3 * time.Second

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
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return backoff.Retry(op, backoff.WithContext(exp, ctx))
}
