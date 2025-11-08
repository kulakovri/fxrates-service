package pg

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgdriver "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//goose:embed all
//go:embed migrations/*.sql
var fs embed.FS

func RunMigrations(ctx context.Context, db *DB) error {
	src, err := iofs.New(fs, "migrations")
	if err != nil {
		return fmt.Errorf("migrate src: %w", err)
	}
	dsn := db.Pool.Config().ConnString()
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql db: %w", err)
	}
	defer sqldb.Close()
	// Retry ping; container might not accept connections immediately
	var pingErr error
	for i := 0; i < 30; i++ {
		pingErr = sqldb.PingContext(ctx)
		if pingErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("ping db: %w", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
	if pingErr != nil {
		return fmt.Errorf("ping db: %w", pingErr)
	}
	driver, err := pgdriver.WithInstance(sqldb, &pgdriver.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
