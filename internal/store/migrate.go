package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func (s *sqlStore) Migrate(ctx context.Context) error {
	dir, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("locate migrations: %w", err)
	}
	dialect := goose.DialectSQLite3
	if s.driver == DriverPostgres {
		dialect = goose.DialectPostgres
	}
	provider, err := goose.NewProvider(dialect, s.db, dir)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
