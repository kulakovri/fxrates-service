package rateclient

import (
	"context"
	"time"

	"fxrates-service/internal/infrastructure/grpc/ratepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn
	cli  ratepb.RateServiceClient
}

func New(ctx context.Context, target string) (*Client, func(), error) {
	conn, err := grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &Client{conn: conn, cli: ratepb.NewRateServiceClient(conn)}, func() { _ = conn.Close() }, nil
}

func (c *Client) Fetch(ctx context.Context, pair, traceID string, timeout time.Duration) (*ratepb.FetchResponse, error) {
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return c.cli.Fetch(ctx, &ratepb.FetchRequest{Pair: pair, TraceId: traceID})
}
