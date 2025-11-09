package logx

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
)

func init() {
	cfg := zap.NewProductionConfig()
	cfg.Sampling = nil
	cfg.DisableStacktrace = true
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if lvl, ok := os.LookupEnv("LOG_LEVEL"); ok {
		if err := cfg.Level.UnmarshalText([]byte(strings.ToLower(lvl))); err == nil {
			cfg.Level = cfg.Level
		}
	}

	var err error
	logger, err = cfg.Build(zap.AddCaller(), zap.AddCallerSkip(0))
	if err != nil {
		panic(err)
	}
}

// L returns the package-level logger instance.
func L() *zap.Logger {
	return logger
}

// WithFields enriches logs with request IDs / trace IDs from context.
// This is a stub implementation that can be extended later.
func WithFields(ctx context.Context) *zap.Logger {
	// TODO: Extract request ID, trace ID, etc. from context
	// For now, return the base logger
	return logger
}
