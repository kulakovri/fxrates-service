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

func NewRouter(s openapi.ServerInterface) http.Handler {
	r := chi.NewRouter()

	r.Use(requestID())
	r.Use(recoverer())
	r.Use(accessLog())

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
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
			logx.L().Info("http_request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", sr.status),
				zap.String("request_id", rid),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}
