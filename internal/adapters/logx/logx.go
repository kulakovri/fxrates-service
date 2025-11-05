package logx

import (
	"context"

	"go.uber.org/zap"
)

var (
	logger *zap.Logger
)

func init() {
	logger = zap.Must(zap.NewProduction())
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
