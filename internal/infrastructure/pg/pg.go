package pg

import (
	"context"
	"time"

	"fxrates-service/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct{ Pool *pgxpool.Pool }

func Connect(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	appCfg := config.Load()
	cfg.MaxConns, cfg.MinConns = int32(appCfg.PGMaxConns), int32(appCfg.PGMinConns)
	cfg.MaxConnIdleTime = 2 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close()                         { d.Pool.Close() }
func (d *DB) Ping(ctx context.Context) error { return d.Pool.Ping(ctx) }
