package store

import (
	"context"
	"testing"
	"time"
)

const migrationBeforeBackfill = 8

func TestMigrationBackfillsPersonalWorkspaces(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			ctx := context.Background()
			s := openMigratedTo(t, target, migrationBeforeBackfill)
			stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
			const userID = "0123456789abcdef0123456789abcdef"
			_, err := s.ExecContext(ctx,
				"INSERT INTO users (id, email, password_hash, is_root, created_at, updated_at) VALUES ($1, $2, $3, TRUE, $4, $4)",
				userID, "alex@example.com", "hash", stamp)
			if err != nil {
				t.Fatalf("insert user: %v", err)
			}
			if err := s.Migrate(ctx); err != nil {
				t.Fatalf("Migrate: %v", err)
			}

			var slug, kind, role, scope string
			err = s.QueryRowContext(ctx,
				`SELECT w.slug, w.kind, a.role, a.scope_type FROM workspaces w
				JOIN workspace_access a ON a.workspace_id = w.id WHERE a.user_id = $1`, userID).Scan(&slug, &kind, &role, &scope)
			if err != nil {
				t.Fatalf("select backfilled workspace: %v", err)
			}
			if slug != "personal-0123456789ab" || kind != "personal" || role != "admin" || scope != "workspace" {
				t.Errorf("backfill = %s %s %s %s", slug, kind, role, scope)
			}
		})
	}
}
