package application

import "context"

// UnitOfWork provides a minimal transaction boundary using context propagation.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// NoopUoW executes the function without starting a transaction.
type NoopUoW struct{}

func (NoopUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
