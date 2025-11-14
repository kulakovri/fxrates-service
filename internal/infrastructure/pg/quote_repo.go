package pg

import (
	"context"

	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/logx"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type QuoteRepo struct{ db *DB }

func NewQuoteRepo(db *DB) *QuoteRepo { return &QuoteRepo{db: db} }

func (r *QuoteRepo) exec(ctx context.Context) execer {
	if tx := txFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db.Pool
}

func (r *QuoteRepo) GetLast(ctx context.Context, pair string) (domain.Quote, error) {
	const q = `SELECT pair, price::float8, updated_at FROM quotes WHERE pair=$1`
	log := logx.L().With(
		zap.String("repo", "quote"),
		zap.String("operation", "GetLast"),
		zap.String("sql", q),
		zap.String("pair", pair),
	)
	log.Info("sql.query_start")
	var out domain.Quote
	if err := r.exec(ctx).QueryRow(ctx, q, pair).Scan(&out.Pair, &out.Price, &out.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			log.Info("sql.query_no_rows")
			return domain.Quote{}, domain.ErrNotFound
		}
		log.Error("sql.query_failed", zap.Error(err))
		return domain.Quote{}, err
	}
	log.Info("sql.query_success",
		zap.Float64("price", out.Price),
		zap.Time("updated_at", out.UpdatedAt),
	)
	return out, nil
}

func (r *QuoteRepo) Upsert(ctx context.Context, q domain.Quote) error {
	const up = `
        INSERT INTO quotes(pair, price, updated_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (pair) DO UPDATE
          SET price=EXCLUDED.price, updated_at=EXCLUDED.updated_at`
	log := logx.L().With(
		zap.String("repo", "quote"),
		zap.String("operation", "Upsert"),
		zap.String("sql", up),
		zap.String("pair", string(q.Pair)),
		zap.Float64("price", q.Price),
		zap.Time("updated_at", q.UpdatedAt),
	)
	log.Info("sql.exec_start")
	tag, err := r.exec(ctx).Exec(ctx, up, q.Pair, q.Price, q.UpdatedAt)
	if err != nil {
		log.Error("sql.exec_failed", zap.Error(err))
		return err
	}
	log.Info("sql.exec_success", zap.Int64("rows_affected", int64(tag.RowsAffected())))
	return nil
}

func (r *QuoteRepo) AppendHistory(ctx context.Context, h domain.QuoteHistory) error {
	const insertHistory = `
        INSERT INTO quotes_history(pair, price, quoted_at, source, update_id)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (pair, quoted_at, source) DO NOTHING
    `
	log := logx.L().With(
		zap.String("repo", "quote"),
		zap.String("operation", "AppendHistory"),
		zap.String("sql", insertHistory),
		zap.String("pair", string(h.Pair)),
		zap.Float64("price", h.Price),
		zap.Time("quoted_at", h.QuotedAt),
		zap.String("source", h.Source),
	)
	if h.UpdateID != nil {
		log = log.With(zap.String("update_id", *h.UpdateID))
	}
	log.Info("sql.exec_start")
	tag, err := r.exec(ctx).Exec(ctx, insertHistory, h.Pair, h.Price, h.QuotedAt, h.Source, h.UpdateID)
	if err != nil {
		log.Error("sql.exec_failed", zap.Error(err))
		return err
	}
	log.Info("sql.exec_success", zap.Int64("rows_affected", int64(tag.RowsAffected())))
	return nil
}
