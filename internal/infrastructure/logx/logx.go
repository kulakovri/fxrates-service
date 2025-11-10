package logx

import (
	"context"
	"strings"

	"fxrates-service/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
)

func init() {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Sampling = nil
	zapCfg.DisableStacktrace = true
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	appCfg := config.Load()
	if appCfg.LogLevel != "" {
		_ = zapCfg.Level.UnmarshalText([]byte(strings.ToLower(appCfg.LogLevel)))
	}

	var err error
	logger, err = zapCfg.Build(zap.AddCaller(), zap.AddCallerSkip(0))
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
