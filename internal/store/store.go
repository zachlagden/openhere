package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverSQLite   Driver = "sqlite"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Tx interface {
	Querier
	Commit() error
	Rollback() error
}

type Store interface {
	Querier
	Driver() Driver
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	Migrate(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error
}

type Options struct {
	DatabaseURL string
	DataDir     string
}

func Open(ctx context.Context, opts Options) (Store, error) {
	if opts.DatabaseURL != "" {
		return openPostgres(ctx, opts.DatabaseURL)
	}
	if opts.DataDir == "" {
		return nil, errors.New("open store: neither a database URL nor a data directory is set")
	}
	return openSQLite(ctx, opts.DataDir)
}

func WithTx(ctx context.Context, s Store, fn func(Querier) error) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("roll back transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
