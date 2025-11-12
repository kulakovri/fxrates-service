package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

func txFromCtx(ctx context.Context) pgx.Tx {
	if v := ctx.Value(txKey{}); v != nil {
		if tx, ok := v.(pgx.Tx); ok {
			return tx
		}
	}
	return nil
}

type UnitOfWork struct {
	Pool *pgxpool.Pool
}

func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := u.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
