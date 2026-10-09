package store

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

const (
	userID    = "0123456789abcdef0123456789abcdef"
	siteID    = "1123456789abcdef0123456789abcdef"
	versionID = "2123456789abcdef0123456789abcdef"
	otherSite = "3123456789abcdef0123456789abcdef"
	otherVer  = "4123456789abcdef0123456789abcdef"
)

func seedSite(t *testing.T, s Store, id, slug string) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if _, err := s.ExecContext(ctx,
		"INSERT INTO users (id, email, password_hash, is_root, created_at, updated_at) VALUES ($1, $2, 'hash', FALSE, $3, $3) ON CONFLICT (id) DO NOTHING",
		userID, "alex@example.com", stamp); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := s.ExecContext(ctx,
		"INSERT INTO workspaces (id, slug, name, kind, created_by, created_at, updated_at) VALUES ($1, 'acme', 'Acme Ltd', 'shared', $2, $3, $3) ON CONFLICT (id) DO NOTHING",
		userID, userID, stamp); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err := s.ExecContext(ctx,
		"INSERT INTO sites (id, workspace_id, slug, display_name, created_at, updated_at) VALUES ($1, $2, $3, $3, $4, $4)",
		id, userID, slug, stamp); err != nil {
		t.Fatalf("insert site: %v", err)
	}
}

func seedVersion(t *testing.T, s Store, id, site string) {
	t.Helper()
	stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if _, err := s.ExecContext(context.Background(),
		`INSERT INTO versions (id, site_id, status, created_at, finalized_at, file_count, total_bytes)
		VALUES ($1, $2, 'finalized', $3, $3, 1, 5)`, id, site, stamp); err != nil {
		t.Fatalf("insert version: %v", err)
	}
}

func count(t *testing.T, s Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestVersionConstraints(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			s := openMigrated(t, target)
			ctx := context.Background()
			seedSite(t, s, siteID, "docs")

			if _, err := s.ExecContext(ctx,
				"INSERT INTO versions (id, site_id, status, created_at, file_count, total_bytes) VALUES ($1, 'missing', 'pending', $2, 0, 0)",
				versionID, time.Now().UTC()); err == nil {
				t.Error("a version for an unknown site was accepted")
			}
			if _, err := s.ExecContext(ctx,
				"INSERT INTO versions (id, site_id, status, created_at, file_count, total_bytes) VALUES ($1, $2, 'live', $3, 0, 0)",
				versionID, siteID, time.Now().UTC()); err == nil {
				t.Error("an unknown status was accepted")
			}
			if _, err := s.ExecContext(ctx,
				"INSERT INTO versions (id, site_id, status, created_at, file_count, total_bytes) VALUES ($1, $2, 'finalized', $3, 0, 0)",
				versionID, siteID, time.Now().UTC()); err == nil {
				t.Error("a finalized version without a finalized_at was accepted")
			}

			seedVersion(t, s, versionID, siteID)
			if _, err := s.ExecContext(ctx,
				"INSERT INTO version_files (version_id, path, size, sha256, content_type) VALUES ($1, 'index.html', 5, $2, 'text/html')",
				versionID, "short"); err == nil {
				t.Error("a malformed sha256 was accepted")
			}
			const hash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
			const insertFile = "INSERT INTO version_files (version_id, path, size, sha256, content_type) VALUES ($1, 'index.html', 5, $2, 'text/html')"
			if _, err := s.ExecContext(ctx, insertFile, versionID, hash); err != nil {
				t.Fatalf("insert file: %v", err)
			}
			if _, err := s.ExecContext(ctx, insertFile, versionID, hash); err == nil {
				t.Error("a duplicate path in one version was accepted")
			}
			if _, err := s.ExecContext(ctx, insertFile, "missing", hash); err == nil {
				t.Error("a file for an unknown version was accepted")
			}
		})
	}
}

func TestSitePointerConstraints(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			s := openMigrated(t, target)
			ctx := context.Background()
			seedSite(t, s, siteID, "docs")
			seedVersion(t, s, versionID, siteID)

			if _, err := s.ExecContext(ctx, "UPDATE sites SET current_version_id = 'missing' WHERE id = $1", siteID); err == nil {
				t.Error("a pointer to an unknown version was accepted")
			}
			if _, err := s.ExecContext(ctx, "UPDATE sites SET current_version_id = $1 WHERE id = $2", versionID, siteID); err != nil {
				t.Fatalf("point site at version: %v", err)
			}
			if _, err := s.ExecContext(ctx, "DELETE FROM versions WHERE id = $1", versionID); err == nil {
				t.Error("the live version was deleted")
			}
			var spa bool
			if err := s.QueryRowContext(ctx, "SELECT spa_mode FROM sites WHERE id = $1", siteID).Scan(&spa); err != nil || spa {
				t.Errorf("spa_mode = %v, %v; want false", spa, err)
			}
		})
	}
}

func TestDeletingASiteRemovesItsVersions(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			s := openMigrated(t, target)
			ctx := context.Background()
			seedSite(t, s, siteID, "docs")
			seedSite(t, s, otherSite, "notes")
			seedVersion(t, s, versionID, siteID)
			seedVersion(t, s, otherVer, otherSite)
			const hash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
			for _, id := range []string{versionID, otherVer} {
				if _, err := s.ExecContext(ctx,
					"INSERT INTO version_files (version_id, path, size, sha256, content_type) VALUES ($1, 'a', 5, $2, 'text/plain')", id, hash); err != nil {
					t.Fatalf("insert file: %v", err)
				}
			}
			if _, err := s.ExecContext(ctx, "UPDATE sites SET current_version_id = $1 WHERE id = $2", versionID, siteID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ExecContext(ctx, "DELETE FROM sites WHERE id = $1", siteID); err != nil {
				t.Fatalf("delete site with a live version: %v", err)
			}
			if n := count(t, s, "SELECT COUNT(*) FROM versions WHERE site_id = $1", siteID); n != 0 {
				t.Errorf("%d versions survived their site", n)
			}
			if n := count(t, s, "SELECT COUNT(*) FROM version_files WHERE version_id = $1", versionID); n != 0 {
				t.Errorf("%d files survived their site", n)
			}
			if n := count(t, s, "SELECT COUNT(*) FROM versions WHERE site_id = $1", otherSite); n != 1 {
				t.Errorf("other site has %d versions, want 1", n)
			}
		})
	}
}

func TestPublishingMigrationsRollBack(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			s := openMigrated(t, target)
			store, ok := s.(*sqlStore)
			if !ok {
				t.Fatalf("store is %T, want *sqlStore", s)
			}
			dir, err := fs.Sub(migrationFiles, "migrations")
			if err != nil {
				t.Fatal(err)
			}
			dialect := goose.DialectSQLite3
			if store.driver == DriverPostgres {
				dialect = goose.DialectPostgres
			}
			provider, err := goose.NewProvider(dialect, store.db, dir)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if _, err := provider.DownTo(ctx, migrationBeforePublishing); err != nil {
				t.Fatalf("roll back: %v", err)
			}
			if _, err := s.ExecContext(ctx, "SELECT current_version_id FROM sites"); err == nil {
				t.Error("current_version_id survived the rollback")
			}
			if err := s.Migrate(ctx); err != nil {
				t.Fatalf("migrate again: %v", err)
			}
			if _, err := s.ExecContext(ctx, "SELECT current_version_id, spa_mode FROM sites"); err != nil {
				t.Errorf("columns missing after migrating again: %v", err)
			}
		})
	}
}

const migrationBeforePublishing = 9
