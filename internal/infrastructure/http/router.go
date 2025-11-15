package httpserver

import (
	"context"
	"net/http"
	"os"
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

	// Use custom error handler to ensure JSON error envelope on binding/validation errors
	openapi.HandlerWithOptions(s, openapi.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadRequest, "bad request")
		},
	})

	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		var data []byte
		var err error
		for _, p := range []string{"api/openapi.yaml", "/usr/local/share/fxrates/openapi.yaml"} {
			data, err = os.ReadFile(p)
			if err == nil {
				break
			}
		}
		if err != nil {
			http.Error(w, "failed to load openapi spec", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	// Serve minimal Swagger UI
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerHTML))
	})
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

const swaggerHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8" />
  <title>fxrates-service API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout",
        requestInterceptor: (req) => {
          // Auto-generate X-Idempotency-Key if missing
          const headers = req.headers || {};
          if (!headers["X-Idempotency-Key"] && !headers["x-idempotency-key"]) {
            if (window.crypto && window.crypto.randomUUID) {
              headers["X-Idempotency-Key"] = window.crypto.randomUUID();
            }
          }
          // Force requests to current origin (avoids hardcoded server URL ports)
          try {
            const u = new URL(req.url, window.location.href);
            u.protocol = window.location.protocol;
            u.host = window.location.host;
            req.url = u.toString();
          } catch (_) { /* ignore */ }
          req.headers = headers;
          return req;
        }
      });
    };
  </script>
</body>
</html>`
