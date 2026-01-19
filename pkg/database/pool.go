package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ilramdhan/costing-mvp/config"
)

// NewPool creates a new PostgreSQL connection pool
func NewPool(ctx context.Context, cfg *config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure pool
	poolConfig.MaxConns = int32(cfg.PoolMax)
	poolConfig.MinConns = int32(cfg.PoolMinConns)
	poolConfig.MaxConnLifetime = cfg.PoolMaxConnLife
	poolConfig.MaxConnIdleTime = 15 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// Close closes the pool
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}
