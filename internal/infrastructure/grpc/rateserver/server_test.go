package rateserver

import (
	"context"
	"net"
	"testing"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/grpc/ratepb"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeFetcher struct{}

func (fakeFetcher) FetchRate(_ context.Context, pair string) (domain.Quote, error) {
	return domain.Quote{
		Pair:      domain.Pair(pair),
		Price:     1.2345,
		UpdatedAt: time.Now(),
	}, nil
}

func TestRateServer_Fetch(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() { _ = lis.Close() })

	s := grpc.NewServer()
	srv := NewServer(application.QuoteFetcher(fakeFetcher{}), zap.NewNop())
	ratepb.RegisterRateServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	// Dial bufconn
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	cli := ratepb.NewRateServiceClient(conn)
	resp, err := cli.Fetch(ctx, &ratepb.FetchRequest{Pair: "EUR/USD", TraceId: "tid-1"})
	require.NoError(t, err)
	require.Equal(t, "EUR/USD", resp.GetPair())
	require.InDelta(t, 1.2345, resp.GetPrice(), 0.000001)
	_, err = time.Parse(time.RFC3339Nano, resp.GetUpdatedAt())
	require.NoError(t, err)
}
