package rateserver

import (
	"context"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/infrastructure/grpc/ratepb"

	"go.uber.org/zap"
)

type Server struct {
	svc application.QuoteFetcher
	log *zap.Logger
	ratepb.UnimplementedRateServiceServer
}

// NewServer wires a fetch-only gRPC server that delegates to the application service.
func NewServer(svc application.QuoteFetcher, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}

func (s *Server) Fetch(ctx context.Context, req *ratepb.FetchRequest) (*ratepb.FetchResponse, error) {
	log := s.log
	pair := req.GetPair()
	traceID := req.GetTraceId()
	log = log.With(zap.String("pair", pair), zap.String("trace_id", traceID))

	q, err := s.svc.FetchQuote(ctx, pair)
	if err != nil {
		log.Warn("grpc_fetch.provider_error", zap.Error(err))
		return nil, err
	}
	log.Info("grpc_fetch.success", zap.Float64("price", q.Price))
	return &ratepb.FetchResponse{
		Pair:      string(q.Pair),
		Price:     q.Price,
		UpdatedAt: q.UpdatedAt.Format(time.RFC3339Nano),
	}, nil
}
