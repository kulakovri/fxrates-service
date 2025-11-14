package redisstore

import "context"

// NoopIdempotency always succeeds; useful for tests/dev when Redis is disabled.
type NoopIdempotency struct{}

func (NoopIdempotency) TryReserve(context.Context, string) (bool, error) { return true, nil }
