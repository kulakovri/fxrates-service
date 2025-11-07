package pg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/pg"
	"github.com/stretchr/testify/require"
)

func TestAppendHistory(t *testing.T) {
	t.Parallel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping PG test")
	}
	ctx := context.Background()
	db, err := pg.Connect(ctx, dsn)
	if err != nil {
		t.Skip("pg not available: ", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Skip("pg not reachable: ", err)
	}

	repo := pg.NewQuoteRepo(db)
	record := domain.QuoteHistory{
		Pair:     "EUR/USD",
		Price:    1.2345,
		QuotedAt: time.Now().UTC(),
		Source:   "test",
	}

	err = repo.AppendHistory(ctx, record)
	require.NoError(t, err)
}
