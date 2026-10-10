package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testTarget struct {
	name string
	opts func(t *testing.T) Options
}

func testTargets() []testTarget {
	targets := []testTarget{{
		name: "sqlite",
		opts: func(t *testing.T) Options { return Options{DataDir: t.TempDir()} },
	}}
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		targets = append(targets, testTarget{
			name: "postgres",
			opts: func(t *testing.T) Options { return Options{DatabaseURL: url} },
		})
	} else {
		targets = append(targets, testTarget{
			name: "postgres",
			opts: func(t *testing.T) Options {
				t.Skip("TEST_DATABASE_URL is not set")
				return Options{}
			},
		})
	}
	return targets
}

func openMigrated(t *testing.T, target testTarget) Store {
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
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}

func cleanPostgres(t *testing.T, s Store) {
	t.Helper()
	for _, table := range []string{"agent_codes", "password_resets", "invites", "viewer_handoffs", "viewer_codes", "site_domains", "site_access_domains", "site_access_emails", "workspace_access", "version_files", "versions", "sites", "folders", "workspaces", "api_keys", "sessions", "users", "instance_settings", "goose_db_version"} {
		if _, err := s.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+table+" CASCADE"); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}
}

func TestStore(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			t.Run("ping and driver", func(t *testing.T) {
				s := openMigrated(t, target)
				if err := s.Ping(context.Background()); err != nil {
					t.Errorf("Ping: %v", err)
				}
				if string(s.Driver()) != target.name {
					t.Errorf("Driver = %s, want %s", s.Driver(), target.name)
				}
			})
			t.Run("migrate is idempotent", func(t *testing.T) {
				s := openMigrated(t, target)
				if err := s.Migrate(context.Background()); err != nil {
					t.Fatalf("second Migrate: %v", err)
				}
				assertSettingsTableEmpty(t, s)
			})
			t.Run("settings round trip", func(t *testing.T) {
				s := openMigrated(t, target)
				assertSettingsRoundTrip(t, s)
			})
			t.Run("transaction commit and rollback", func(t *testing.T) {
				s := openMigrated(t, target)
				assertTransactions(t, s)
			})
		})
	}
}

func assertSettingsTableEmpty(t *testing.T, s Store) {
	t.Helper()
	var count int
	if err := s.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM instance_settings").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

const upsertSetting = `INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`

func assertSettingsRoundTrip(t *testing.T, s Store) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	if _, err := s.ExecContext(ctx, upsertSetting, "site_limit", "10", stamp); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := s.ExecContext(ctx, upsertSetting, "site_limit", "20", stamp.Add(time.Hour)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var value string
	var updatedAt time.Time
	row := s.QueryRowContext(ctx, "SELECT value, updated_at FROM instance_settings WHERE key = $1", "site_limit")
	if err := row.Scan(&value, &updatedAt); err != nil {
		t.Fatalf("select: %v", err)
	}
	if value != "20" {
		t.Errorf("value = %q, want 20", value)
	}
	if !updatedAt.Equal(stamp.Add(time.Hour)) {
		t.Errorf("updated_at = %v, want %v", updatedAt, stamp.Add(time.Hour))
	}
}

func assertTransactions(t *testing.T, s Store) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Now().UTC()

	rollbackCause := errors.New("stop")
	err := WithTx(ctx, s, func(q Querier) error {
		if _, err := q.ExecContext(ctx, upsertSetting, "rolled_back", "x", stamp); err != nil {
			return err
		}
		return rollbackCause
	})
	if !errors.Is(err, rollbackCause) {
		t.Fatalf("WithTx error = %v, want %v", err, rollbackCause)
	}
	assertSettingsTableEmpty(t, s)

	err = WithTx(ctx, s, func(q Querier) error {
		_, err := q.ExecContext(ctx, upsertSetting, "committed", "y", stamp)
		return err
	})
	if err != nil {
		t.Fatalf("WithTx commit: %v", err)
	}
	var value string
	if err := s.QueryRowContext(ctx, "SELECT value FROM instance_settings WHERE key = $1", "committed").Scan(&value); err != nil {
		t.Fatalf("select committed: %v", err)
	}
	if value != "y" {
		t.Errorf("value = %q, want y", value)
	}
}

func TestOpenSQLiteCreatesDatabaseFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	s, err := Open(context.Background(), Options{DataDir: dir})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, sqliteFileName)); err != nil {
		t.Errorf("database file missing: %v", err)
	}
}

func TestOpenRequiresATarget(t *testing.T) {
	if _, err := Open(context.Background(), Options{}); err == nil {
		t.Error("expected an error with no database URL and no data directory")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			s := openMigrated(t, target)
			ctx := context.Background()
			stamp := time.Now().UTC()
			const insert = "INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)"
			if _, err := s.ExecContext(ctx, insert, "dup", "a", stamp); err != nil {
				t.Fatalf("first insert: %v", err)
			}
			_, err := s.ExecContext(ctx, insert, "dup", "b", stamp)
			if err == nil {
				t.Fatal("expected a duplicate key error")
			}
			if !IsUniqueViolation(err) {
				t.Errorf("IsUniqueViolation(%v) = false, want true", err)
			}
			if IsUniqueViolation(errors.New("other")) {
				t.Error("IsUniqueViolation reported a plain error as a unique violation")
			}
		})
	}
}
