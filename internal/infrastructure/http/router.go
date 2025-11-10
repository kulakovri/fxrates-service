package httpserver

import (
	"context"
	"net/http"
	"time"

	"fxrates-service/internal/infrastructure/http/openapi"
	"fxrates-service/internal/infrastructure/logx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const traceIDKey contextKey = "trace_id"

func NewRouter(s *Server) http.Handler {
	r := chi.NewRouter()

	r.Use(requestID())
	r.Use(traceID())
	r.Use(recoverer())
	r.Use(accessLog())

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.ping != nil {
			if err := s.ping(r.Context()); err != nil {
				writeError(w, http.StatusServiceUnavailable, "db not ready")
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})

	openapi.HandlerFromMux(s, r)
	return r
}

func requestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get("X-Request-ID")
			if rid == "" {
				rid = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", rid)
			ctx := context.WithValue(r.Context(), requestIDKey, rid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func getTraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

func recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					rid, _ := r.Context().Value(requestIDKey).(string)
					logx.L().Error("panic recovered", zap.Any("error", rec), zap.String("request_id", rid))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	n, err := sr.ResponseWriter.Write(b)
	sr.bytes += n
	return n, err
}

func accessLog() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(sr, r)
			rid, _ := r.Context().Value(requestIDKey).(string)
			tid, _ := r.Context().Value(traceIDKey).(string)
			logx.L().Info("http_request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", sr.status),
				zap.String("request_id", rid),
				zap.String("trace_id", tid),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

func traceID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid := r.Header.Get("X-Trace-Id")
			if tid == "" {
				tid = uuid.NewString()
			}
			w.Header().Set("X-Trace-Id", tid)
			ctx := context.WithValue(r.Context(), traceIDKey, tid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
