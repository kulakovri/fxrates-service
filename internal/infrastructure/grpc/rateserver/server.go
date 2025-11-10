package rateserver

import (
	"context"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/grpc/ratepb"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	RP  application.RateProvider
	Log *zap.Logger
	ratepb.UnimplementedRateServiceServer
}

func (s *Server) Fetch(ctx context.Context, req *ratepb.FetchRequest) (*ratepb.FetchResponse, error) {
	log := s.Log
	if log == nil {
		log = zap.NewNop()
	}
	pair := req.GetPair()
	traceID := req.GetTraceId()
	log = log.With(zap.String("pair", pair), zap.String("trace_id", traceID))

	if !domain.ValidatePair(pair) {
		log.Warn("grpc_fetch.invalid_pair")
		return nil, status.Error(codes.InvalidArgument, "unsupported pair")
	}

	q, err := s.RP.Get(ctx, pair)
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
