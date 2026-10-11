package workspaces

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zachlagden/openhere/internal/ids"
	"github.com/zachlagden/openhere/internal/store"
)

func nullable(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func insertGrant(ctx context.Context, q store.Querier, g Grant, at time.Time) (Grant, error) {
	id, err := ids.New()
	if err != nil {
		return Grant{}, err
	}
	g.ID = id
	_, err = q.ExecContext(ctx,
		`INSERT INTO workspace_access (id, workspace_id, user_id, role, scope_type, folder_id, site_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		g.ID, g.WorkspaceID, g.UserID, string(g.Role), string(g.ScopeType), nullable(g.FolderID), nullable(g.SiteID), at)
	if err != nil {
		return Grant{}, fmt.Errorf("insert grant: %w", err)
	}
	return g, nil
}

func lockWorkspace(ctx context.Context, q store.Querier, workspaceID string) error {
	if _, err := q.ExecContext(ctx, "UPDATE workspaces SET updated_at = updated_at WHERE id = $1", workspaceID); err != nil {
		return fmt.Errorf("lock workspace: %w", err)
	}
	return nil
}
