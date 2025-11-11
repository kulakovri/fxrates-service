package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/config"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/grpc/ratepb"
	"fxrates-service/internal/infrastructure/http/openapi"
	"fxrates-service/internal/infrastructure/logx"

	"go.uber.org/zap"
)

// RateFetcher is the minimal interface needed from the gRPC client.
type RateFetcher interface {
	Fetch(ctx context.Context, pair, traceID string, timeout time.Duration) (*ratepb.FetchResponse, error)
}

type Server struct {
	svc        *application.FXRatesService
	ping       func(context.Context) error
	quoteRepo  application.QuoteRepo
	jobRepo    application.UpdateJobRepo
	grpcClient RateFetcher
	cfg        config.Config
	enqueue    func(ctx context.Context, id, pair, traceID string) error
}

func NewServer(svc *application.FXRatesService) *Server { return &Server{svc: svc} }

func (s *Server) SetReadyCheck(fn func(context.Context) error) { s.ping = fn }

func (s *Server) SetEnqueuer(fn func(context.Context, string, string, string) error) {
	s.enqueue = fn
}

// AttachGRPCBackground enables async background processing via gRPC worker mode.
func (s *Server) AttachGRPCBackground(q application.QuoteRepo, u application.UpdateJobRepo, c RateFetcher, cfg config.Config) {
	s.quoteRepo = q
	s.jobRepo = u
	s.grpcClient = c
	s.cfg = cfg
}

func (s *Server) RequestQuoteUpdate(w http.ResponseWriter, r *http.Request, params openapi.RequestQuoteUpdateParams) {
	log := loggerForRequest(r)
	var body openapi.QuoteUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("request_quote_update.decode_failed", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Pair == "" {
		log.Warn("request_quote_update.missing_pair")
		writeError(w, http.StatusBadRequest, "pair is required")
		return
	}
	if !domain.ValidatePair(body.Pair) {
		log.Warn("request_quote_update.invalid_pair_format", zap.String("pair", body.Pair))
		writeError(w, http.StatusBadRequest, "invalid pair")
		return
	}
	idem := r.Header.Get("X-Idempotency-Key")
	if idem == "" {
		log.Warn("request_quote_update.missing_idem")
		writeError(w, http.StatusBadRequest, "X-Idempotency-Key is required")
		return
	}
	log = log.With(
		zap.String("pair", body.Pair),
		zap.String("idempotency_key", idem),
	)
	log.Info("request_quote_update.call_service")
	id, err := s.svc.RequestQuoteUpdate(r.Context(), body.Pair, &idem)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrBadRequest):
			writeError(w, http.StatusBadRequest, "bad request")
			return
		case errors.Is(err, domain.ErrUnsupportedPair):
			writeError(w, http.StatusBadRequest, "invalid pair")
			return
		case errors.Is(err, application.ErrConflict):
			writeError(w, http.StatusConflict, "conflict")
			return
		default:
			logRequestError(r, "request quote update failed", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	log.Info("request_quote_update.queued", zap.String("update_id", id))
	resp := openapi.QuoteUpdateResponse{UpdateId: id}
	writeJSON(w, http.StatusAccepted, resp)

	// In-Process chan mode: enqueue to channel if available.
	if s.enqueue != nil {
		traceID := getTraceIDFromContext(r.Context())
		if err := s.enqueue(r.Context(), id, body.Pair, traceID); err != nil {
			writeError(w, http.StatusServiceUnavailable, "queue busy")
			return
		}
		// In gRPC mode, trigger background fetch via gRPC and persist results.
	} else if s.cfg.WorkerType == "grpc" && s.grpcClient != nil && s.quoteRepo != nil && s.jobRepo != nil {
		traceID := getTraceIDFromContext(r.Context())
		pair := body.Pair
		updateID := id
		timeout := s.cfg.RequestTimeout
		go s.svc.ProcessGRPCUpdate(context.Background(), updateID, pair, func(ctx context.Context) (domain.Quote, error) {
			l := logx.L().With(zap.String("trace_id", traceID), zap.String("pair", pair), zap.String("update_id", updateID))
			res, err := s.grpcClient.Fetch(ctx, pair, traceID, timeout)
			if err != nil {
				l.Warn("grpc_background.fetch_failed", zap.Error(err))
				return domain.Quote{}, err
			}
			t, err := time.Parse(time.RFC3339Nano, res.GetUpdatedAt())
			if err != nil {
				l.Warn("grpc_background.bad_time", zap.Error(err))
				return domain.Quote{}, err
			}
			return domain.Quote{
				Pair:      domain.Pair(res.GetPair()),
				Price:     res.GetPrice(),
				UpdatedAt: t,
			}, nil
		})
	}
}

func (s *Server) GetQuoteUpdate(w http.ResponseWriter, r *http.Request, id string) {
	log := loggerForRequest(r).With(zap.String("update_id", id))
	log.Info("get_quote_update.call_service")
	upd, err := s.svc.GetQuoteUpdate(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Info("get_quote_update.not_found")
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		logRequestError(r, "get quote update failed", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	log.Info("get_quote_update.success", zap.String("status", string(upd.Status)))
	// Map price (*float64) to OpenAPI price (*float32)
	var price *float32
	if upd.Price != nil {
		p := float32(*upd.Price)
		price = &p
	}
	resp := openapi.QuoteUpdateDetails{
		UpdateId:  upd.ID,
		Pair:      string(upd.Pair),
		Status:    mapStatus(upd.Status),
		Price:     price,
		UpdatedAt: upd.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) GetLastQuote(w http.ResponseWriter, r *http.Request, params openapi.GetLastQuoteParams) {
	log := loggerForRequest(r).With(zap.String("pair", params.Pair))
	if !domain.ValidatePair(params.Pair) {
		log.Warn("get_last_quote.invalid_pair_format")
		writeError(w, http.StatusBadRequest, "invalid pair")
		return
	}
	log.Info("get_last_quote.call_service")
	q, err := s.svc.GetLastQuote(r.Context(), params.Pair)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Info("get_last_quote.not_found")
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		logRequestError(r, "get last quote failed", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	log.Info("get_last_quote.success", zap.Float64("price", q.Price))
	var price *float32
	if q.Price != 0 {
		p := float32(q.Price)
		price = &p
	}
	resp := openapi.LastQuote{
		Pair:      string(q.Pair),
		Price:     price,
		UpdatedAt: q.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

// Run starts the HTTP server and blocks until the context is canceled or the server stops.
func (s *Server) Run(ctx context.Context) error {
	addr := ":" + config.Load().Port
	server := &http.Server{
		Addr:    addr,
		Handler: NewRouter(s),
	}
	logx.L().Info("server started", zap.String("addr", addr))
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- http.ErrServerClosed
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.Load().ShutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		logx.L().Info("server stopped")
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(apiError{Code: code, Message: msg})
}

func logRequestError(r *http.Request, msg string, err error) {
	if err == nil {
		return
	}
	loggerForRequest(r).Error(msg,
		zap.Error(err),
	)
}

func loggerForRequest(r *http.Request) *zap.Logger {
	rid, _ := r.Context().Value(requestIDKey).(string)
	tid := r.Header.Get("X-Trace-Id")
	return logx.L().With(
		zap.String("request_id", rid),
		zap.String("trace_id", tid),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
}

func mapStatus(s domain.QuoteUpdateStatus) openapi.QuoteUpdateDetailsStatus {
	switch s {
	case domain.QuoteUpdateStatusQueued:
		return openapi.Queued
	case domain.QuoteUpdateStatusProcessing:
		return openapi.Processing
	case domain.QuoteUpdateStatusDone:
		return openapi.Done
	default:
		return openapi.Failed
	}
}
