package database

import (
	"context"
	"database/sql"
)

// DBInterface is the interface that defines the methods required for database operations
type DBInterface interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// RowScanner is the interface that defines the Scan method
type RowScanner interface {
	Scan(dest ...interface{}) error
}

// sqlDB is the implementation of DBInterface
type sqlDB struct {
	db *sql.DB
}

// NewSQLDB creates a new sqlDB instance
func NewSQLDB(db *sql.DB) DBInterface {
	return &sqlDB{db: db}
}

func (s *sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *sqlDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}
