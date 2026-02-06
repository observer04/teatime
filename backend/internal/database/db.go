package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Common database errors
var (
	ErrNotFound = errors.New("record not found")
)

// Default timeouts for database operations
const (
	DefaultQueryTimeout = 5 * time.Second
	DefaultTxTimeout    = 10 * time.Second
)

// WithTimeout creates a context with the default query timeout
func WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DefaultQueryTimeout)
}

// WithTxTimeout creates a context with transaction timeout
func WithTxTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DefaultTxTimeout)
}

// DB wraps the connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new database connection pool
func New(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	// Connection pool settings
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the connection pool
func (db *DB) Close() {
	db.Pool.Close()
}

// Health checks if database is reachable
func (db *DB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
