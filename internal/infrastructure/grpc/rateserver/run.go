package rateserver

import (
	"context"
	"net"

	"fxrates-service/internal/infrastructure/grpc/ratepb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RunServer starts a gRPC server and blocks until context is done.
func RunServer(ctx context.Context, addr string, srv ratepb.RateServiceServer, log *zap.Logger) error {
	if log == nil {
		log = zap.NewNop()
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	gs := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	ratepb.RegisterRateServiceServer(gs, srv)
	errCh := make(chan error, 1)
	go func() {
		log.Info("grpc_server_started", zap.String("addr", addr))
		if err := gs.Serve(lis); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		log.Info("grpc_server_stopping")
		gs.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}
