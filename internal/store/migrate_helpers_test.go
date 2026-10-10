package store

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func openMigratedTo(t *testing.T, target testTarget, version int64) Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, target.opts(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if target.name == "postgres" {
			cleanPostgres(t, s)
		}
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if target.name == "postgres" {
		cleanPostgres(t, s)
	}
	impl, ok := s.(*sqlStore)
	if !ok {
		t.Fatalf("store is %T, want *sqlStore", s)
	}
	dir, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		t.Fatalf("locate migrations: %v", err)
	}
	dialect := goose.DialectSQLite3
	if impl.driver == DriverPostgres {
		dialect = goose.DialectPostgres
	}
	provider, err := goose.NewProvider(dialect, impl.db, dir)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if _, err := provider.UpTo(ctx, version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	return s
}
