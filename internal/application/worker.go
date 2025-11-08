package application

import "context"

// Worker represents a background processor of jobs.
// Implementations must run until the context is canceled.
type Worker interface {
	Start(ctx context.Context)
}
