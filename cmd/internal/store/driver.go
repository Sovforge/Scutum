package store

import (
	"context"
	"database/sql"
)

// Driver abstracts database-specific behaviour away from Store.
type Driver interface {
	// Open returns a configured *sql.DB for this backend.
	Open(dsn string) (*sql.DB, error)
	// Migrate runs schema creation for this backend.
	Migrate(ctx context.Context, db *sql.DB) error
	// Placeholder returns the positional placeholder for argument n (1-indexed).
	// SQLite/MySQL use "?", PostgreSQL uses "$1", "$2", etc.
	Placeholder(n int) string
}
