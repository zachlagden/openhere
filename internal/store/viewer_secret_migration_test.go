package store

import (
	"context"
	"testing"
	"time"
)

func TestMigrationDeletesTheStoredViewerSecret(t *testing.T) {
	for _, target := range testTargets() {
		t.Run(target.name, func(t *testing.T) {
			ctx := context.Background()
			s := openMigratedTo(t, target, 19)
			stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
			for _, key := range []string{"internal.viewerSecret", "sites.defaultAccessMode"} {
				if _, err := s.ExecContext(ctx, "INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)", key, "00", stamp); err != nil {
					t.Fatalf("insert %s: %v", key, err)
				}
			}
			if err := s.Migrate(ctx); err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			var kept, removed int
			if err := s.QueryRowContext(ctx, "SELECT COUNT(*) FROM instance_settings WHERE key = 'sites.defaultAccessMode'").Scan(&kept); err != nil {
				t.Fatal(err)
			}
			if err := s.QueryRowContext(ctx, "SELECT COUNT(*) FROM instance_settings WHERE key = 'internal.viewerSecret'").Scan(&removed); err != nil {
				t.Fatal(err)
			}
			if kept != 1 || removed != 0 {
				t.Errorf("after migration kept = %d, viewer secret rows = %d; want 1 and 0", kept, removed)
			}
		})
	}
}
