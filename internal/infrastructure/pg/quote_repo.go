package pg

import (
	"context"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
)

type QuoteRepo struct{ db *DB }

func NewQuoteRepo(db *DB) *QuoteRepo { return &QuoteRepo{db: db} }

func (r *QuoteRepo) GetLast(ctx context.Context, pair string) (domain.Quote, error) {
	const q = `SELECT pair, price::float8, updated_at FROM quotes WHERE pair=$1`
	var out domain.Quote
	if err := r.db.Pool.QueryRow(ctx, q, pair).Scan(&out.Pair, &out.Price, &out.UpdatedAt); err != nil {
		return domain.Quote{}, application.ErrNotFound
	}
	return out, nil
}

func (r *QuoteRepo) Upsert(ctx context.Context, q domain.Quote) error {
	const up = `
        INSERT INTO quotes(pair, price, updated_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (pair) DO UPDATE
          SET price=EXCLUDED.price, updated_at=EXCLUDED.updated_at`
	_, err := r.db.Pool.Exec(ctx, up, q.Pair, q.Price, q.UpdatedAt)
	return err
}

func (r *QuoteRepo) AppendHistory(ctx context.Context, h domain.QuoteHistory) error {
	_, err := r.db.Pool.Exec(ctx, `
        INSERT INTO quotes_history(pair, price, quoted_at, source, update_id)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (pair, quoted_at, source) DO NOTHING
    `, h.Pair, h.Price, h.QuotedAt, h.Source, h.UpdateID)
	return err
}
