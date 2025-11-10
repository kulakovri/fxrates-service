package pg

import (
	"context"
	"errors"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/logx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type UpdateJobRepo struct{ db *DB }

func NewUpdateJobRepo(db *DB) *UpdateJobRepo { return &UpdateJobRepo{db: db} }

func (r *UpdateJobRepo) CreateQueued(ctx context.Context, pair string, _ *string) (string, error) {
	id := uuid.NewString()
	const ins = `
        INSERT INTO quote_updates(id, pair, status)
        VALUES ($1, $2, 'queued')`
	log := logx.L().With(
		zap.String("repo", "update_job"),
		zap.String("operation", "CreateQueued"),
		zap.String("sql", ins),
		zap.String("id", id),
		zap.String("pair", pair),
	)
	log.Info("sql.exec_start")
	tag, err := r.db.Pool.Exec(ctx, ins, id, pair)
	if err != nil {
		log.Error("sql.exec_failed", zap.Error(err))
		return "", err
	}
	log.Info("sql.exec_success", zap.Int64("rows_affected", int64(tag.RowsAffected())))
	return id, nil
}

func (r *UpdateJobRepo) GetByID(ctx context.Context, id string) (domain.QuoteUpdate, error) {
	const q = `
        SELECT id::text, pair, status, error, COALESCE(completed_at, requested_at)
        FROM quote_updates WHERE id=$1`
	log := logx.L().With(
		zap.String("repo", "update_job"),
		zap.String("operation", "GetByID"),
		zap.String("sql", q),
		zap.String("id", id),
	)
	log.Info("sql.query_start")
	var out domain.QuoteUpdate
	var errMsg *string
	var status string
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(&out.ID, &out.Pair, &status, &errMsg, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Info("sql.query_no_rows")
		return domain.QuoteUpdate{}, application.ErrNotFound
	}
	if err != nil {
		log.Error("sql.query_failed", zap.Error(err))
		return domain.QuoteUpdate{}, err
	}
	out.Error = errMsg
	switch status {
	case "queued":
		out.Status = domain.QuoteUpdateStatusQueued
	case "processing":
		out.Status = domain.QuoteUpdateStatusProcessing
	case "done":
		out.Status = domain.QuoteUpdateStatusDone
	default:
		out.Status = domain.QuoteUpdateStatusFailed
	}
	log.Info("sql.query_success",
		zap.String("pair", string(out.Pair)),
		zap.String("status", string(out.Status)),
	)
	return out, nil
}

func (r *UpdateJobRepo) UpdateStatus(ctx context.Context, id string, st domain.QuoteUpdateStatus, errMsg *string) error {
	var s string
	switch st {
	case domain.QuoteUpdateStatusQueued:
		s = "queued"
	case domain.QuoteUpdateStatusProcessing:
		s = "processing"
	case domain.QuoteUpdateStatusDone:
		s = "done"
	default:
		s = "failed"
	}
	const up = `
        UPDATE quote_updates
        SET status=$2,
            error=$3,
            completed_at = CASE WHEN $2 IN ('done','failed') THEN NOW() ELSE completed_at END
        WHERE id=$1`
	log := logx.L().With(
		zap.String("repo", "update_job"),
		zap.String("operation", "UpdateStatus"),
		zap.String("sql", up),
		zap.String("id", id),
		zap.String("status", s),
	)
	if errMsg != nil {
		log = log.With(zap.String("error", *errMsg))
	}
	log.Info("sql.exec_start")
	tag, err := r.db.Pool.Exec(ctx, up, id, s, errMsg)
	if err != nil {
		log.Error("sql.exec_failed", zap.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		log.Warn("sql.exec_no_rows")
		return application.ErrNotFound
	}
	log.Info("sql.exec_success", zap.Int64("rows_affected", int64(tag.RowsAffected())))
	return nil
}

func (r *UpdateJobRepo) ClaimQueued(ctx context.Context, limit int) ([]struct{ ID, Pair string }, error) {
	const q = `
      WITH cte AS (
        SELECT id
        FROM quote_updates
        WHERE status = 'queued'
        ORDER BY requested_at
        LIMIT $1
        FOR UPDATE SKIP LOCKED
      )
      UPDATE quote_updates q
      SET status = 'processing'
      FROM cte
      WHERE q.id = cte.id
      RETURNING q.id, q.pair;
    `
	rows, err := r.db.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ ID, Pair string }
	for rows.Next() {
		var id, pair string
		if err := rows.Scan(&id, &pair); err != nil {
			return nil, err
		}
		out = append(out, struct{ ID, Pair string }{ID: id, Pair: pair})
	}
	return out, rows.Err()
}
