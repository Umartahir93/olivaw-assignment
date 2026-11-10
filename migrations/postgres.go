package migrations

import (
	"context"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

// Connection pool limits. Postgres allows 100 connections by default; one
// server stays well below, and idle connections are closed after a while.
const (
	maxOpenConns    = 25
	maxIdleConns    = 25
	connMaxIdleTime = 5 * time.Minute
	connMaxLifetime = time.Hour
)

// Open connects to the database at url, limits its connection pool and
// checks that it answers.
func Open(ctx context.Context, url string) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, "pgx", url)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)
	db.SetConnMaxLifetime(connMaxLifetime)
	return db, nil
}

// Migrate applies every migration that has not been applied yet.
func Migrate(ctx context.Context, db *sqlx.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db.DB, FS)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
