package application

import "context"

// IdempotencyStore handles short-lived request deduplication.
type IdempotencyStore interface {
	// TryReserve returns true if key was absent and is now reserved.
	// Returns false if the key already exists (duplicate).
	TryReserve(ctx context.Context, key string) (bool, error)
}

// NoopIdempotency always succeeds; useful for tests/dev when Redis is disabled.
type NoopIdempotency struct{}

func (NoopIdempotency) TryReserve(context.Context, string) (bool, error) { return true, nil }
