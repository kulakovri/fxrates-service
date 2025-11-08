package pg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"fxrates-service/internal/infrastructure/pg"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func withPostgres(t *testing.T) (*pg.DB, func()) {
	t.Helper()
	if os.Getenv("TESTCONTAINERS") == "" {
		t.Skip("set TESTCONTAINERS=1 to run containerized PG tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	container, err := postgres.RunContainer(ctx,
		postgres.WithDatabase("fxrates"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
	)
	require.NoError(t, err)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := pg.Connect(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pg.RunMigrations(ctx, db))

	teardown := func() {
		db.Close()
		_ = container.Terminate(context.Background())
	}
	return db, teardown
}
