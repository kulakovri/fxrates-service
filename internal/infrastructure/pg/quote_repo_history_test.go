package pg_test

import (
	"context"
	"testing"
	"time"

	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/pg"
	"github.com/stretchr/testify/require"
)

func TestQuoteRepo_AppendHistory_WithContainer(t *testing.T) {
	db, done := withPostgres(t)
	defer done()

	repo := pg.NewQuoteRepo(db)
	ctx := context.Background()

	h := domain.QuoteHistory{
		Pair:     "EUR/USD",
		Price:    1.234567,
		QuotedAt: time.Now().UTC(),
		Source:   "test",
	}
	require.NoError(t, repo.AppendHistory(ctx, h))
	require.NoError(t, repo.AppendHistory(ctx, h))
}

func TestQuoteRepo_UpsertAndGetLast_WithContainer(t *testing.T) {
	db, done := withPostgres(t)
	defer done()

	repo := pg.NewQuoteRepo(db)
	ctx := context.Background()

	q := domain.Quote{Pair: "EUR/USD", Price: 1.111111, UpdatedAt: time.Now().UTC()}
	require.NoError(t, repo.Upsert(ctx, q))

	got, err := repo.GetLast(ctx, "EUR/USD")
	require.NoError(t, err)
	require.Equal(t, q.Pair, got.Pair)
	require.InDelta(t, q.Price, got.Price, 1e-9)
}
