package store

import (
	"context"
	"database/sql"
	"fmt"
)

type rebinder func(query string) string

type sqlStore struct {
	db     *sql.DB
	driver Driver
	rebind rebinder

	onClose func()
}

func identity(query string) string { return query }

func (s *sqlStore) Driver() Driver { return s.driver }

func (s *sqlStore) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.rebind(query), args...)
}

func (s *sqlStore) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.rebind(query), args...)
}

func (s *sqlStore) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.rebind(query), args...)
}

func (s *sqlStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx, rebind: s.rebind}, nil
}

func (s *sqlStore) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping %s: %w", s.driver, err)
	}
	return nil
}

func (s *sqlStore) Close() error {
	err := s.db.Close()
	if s.onClose != nil {
		s.onClose()
	}
	return err
}

type sqlTx struct {
	tx     *sql.Tx
	rebind rebinder
}

func (t *sqlTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, t.rebind(query), args...)
}

func (t *sqlTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, t.rebind(query), args...)
}

func (t *sqlTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, t.rebind(query), args...)
}

func (t *sqlTx) Commit() error   { return t.tx.Commit() }
func (t *sqlTx) Rollback() error { return t.tx.Rollback() }
