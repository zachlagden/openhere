package auth

import (
	"context"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func countRows(t *testing.T, db store.Store, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return count
}

func TestUsersGetAPersonalWorkspace(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		root, err := svc.Setup(ctx, "alex@example.com", testPassword)
		if err != nil {
			t.Fatalf("Setup: %v", err)
		}
		member, err := svc.CreateUser(ctx, "sam@example.com", testPassword)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		for _, user := range []User{root, member} {
			const query = `SELECT COUNT(*) FROM workspaces w JOIN workspace_access a ON a.workspace_id = w.id
				WHERE w.kind = 'personal' AND w.created_by = $1 AND a.user_id = $1 AND a.role = 'admin' AND a.scope_type = 'workspace'`
			if got := countRows(t, db, query, user.ID); got != 1 {
				t.Errorf("personal workspaces for %s = %d, want 1", user.Email, got)
			}
			if got := countRows(t, db, "SELECT COUNT(*) FROM workspace_access WHERE workspace_id = $1", user.ID); got != 1 {
				t.Errorf("members of the personal workspace of %s = %d, want 1", user.Email, got)
			}
		}
	})
}

func TestFailedWorkspaceCreationRollsBackTheUser(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		if _, err := db.ExecContext(ctx, "ALTER TABLE workspaces RENAME TO workspaces_unavailable"); err != nil {
			t.Fatalf("rename workspaces: %v", err)
		}

		if _, err := svc.CreateUser(ctx, "sam@example.com", testPassword); err == nil {
			t.Error("CreateUser succeeded without a workspaces table")
		}
		if _, err := svc.Setup(ctx, "alex@example.com", testPassword); err == nil {
			t.Error("Setup succeeded without a workspaces table")
		}
		if got := countRows(t, db, "SELECT COUNT(*) FROM users"); got != 0 {
			t.Errorf("users left behind = %d, want 0", got)
		}
		needed, err := svc.SetupNeeded(ctx)
		if err != nil || !needed {
			t.Errorf("SetupNeeded = %v, %v; want true after the failed setup", needed, err)
		}
	})
}
