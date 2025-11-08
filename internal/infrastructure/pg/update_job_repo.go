package pg

import (
	"context"
	"errors"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UpdateJobRepo struct{ db *DB }

func NewUpdateJobRepo(db *DB) *UpdateJobRepo { return &UpdateJobRepo{db: db} }

func (r *UpdateJobRepo) CreateQueued(ctx context.Context, pair string, idem *string) (string, error) {
	if idem != nil {
		const findByIdem = `SELECT id::text FROM quote_updates WHERE idempotency_key=$1`
		var existing string
		if err := r.db.Pool.QueryRow(ctx, findByIdem, *idem).Scan(&existing); err == nil {
			return existing, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}

	id := uuid.NewString()
	const ins = `
        INSERT INTO quote_updates(id, pair, status, idempotency_key)
        VALUES ($1, $2, 'queued', $3)`
	if _, err := r.db.Pool.Exec(ctx, ins, id, pair, idem); err != nil {
		if idem != nil {
			const back = `SELECT id::text FROM quote_updates WHERE idempotency_key=$1`
			var existing string
			if e := r.db.Pool.QueryRow(ctx, back, *idem).Scan(&existing); e == nil {
				return existing, nil
			}
		}
		return "", err
	}
	return id, nil
}

func (r *UpdateJobRepo) GetByID(ctx context.Context, id string) (domain.QuoteUpdate, error) {
	const q = `
        SELECT id::text, pair, status, error, COALESCE(completed_at, requested_at)
        FROM quote_updates WHERE id=$1`
	var out domain.QuoteUpdate
	var errMsg *string
	var status string
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(&out.ID, &out.Pair, &status, &errMsg, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QuoteUpdate{}, application.ErrNotFound
	}
	if err != nil {
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
	ct, err := r.db.Pool.Exec(ctx, up, id, s, errMsg)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return application.ErrNotFound
	}
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
