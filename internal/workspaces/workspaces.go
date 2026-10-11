package workspaces

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/validate"
)

func (s *Service) List(ctx context.Context, userID string) ([]Access, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+workspaceColumns+`, a.id, a.workspace_id, a.user_id, a.role, a.scope_type, a.folder_id, a.site_id
		FROM workspaces w JOIN workspace_access a ON a.workspace_id = w.id
		WHERE a.user_id = $1
		ORDER BY w.kind, w.name, w.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	result := []Access{}
	for rows.Next() {
		var ws Workspace
		var g Grant
		if err := scanWorkspaceGrant(rows, &ws, &g); err != nil {
			return nil, errors.Join(fmt.Errorf("scan workspace: %w", err), rows.Close())
		}
		if n := len(result); n > 0 && result[n-1].Workspace.ID == ws.ID {
			result[n-1].Grants = append(result[n-1].Grants, g)
			continue
		}
		result = append(result, Access{Workspace: ws, Grants: []Grant{g}})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read workspaces: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close workspace rows: %w", err)
	}
	return result, nil
}

func scanWorkspaceGrant(row scanner, ws *Workspace, g *Grant) error {
	var createdBy, folderID, siteID sql.NullString
	err := row.Scan(&ws.ID, &ws.Slug, &ws.Name, &ws.Kind, &createdBy, &ws.CreatedAt, &ws.UpdatedAt,
		&g.ID, &g.WorkspaceID, &g.UserID, &g.Role, &g.ScopeType, &folderID, &siteID)
	if err != nil {
		return err
	}
	ws.CreatedBy = createdBy.String
	g.FolderID = folderID.String
	g.SiteID = siteID.String
	return nil
}

func (s *Service) Rename(ctx context.Context, userID, ref, name string) (Access, error) {
	acc, err := s.Authorize(ctx, userID, ref, RoleAdmin)
	if err != nil {
		return Access{}, err
	}
	clean, err := validate.Line("Name", name, MaxNameLength)
	if err != nil {
		return Access{}, err
	}
	now := s.clock()
	if _, err := s.db.ExecContext(ctx, "UPDATE workspaces SET name = $1, updated_at = $2 WHERE id = $3", clean, now, acc.Workspace.ID); err != nil {
		return Access{}, fmt.Errorf("rename workspace: %w", err)
	}
	acc.Workspace.Name = clean
	acc.Workspace.UpdatedAt = now
	return acc, nil
}

func (s *Service) Delete(ctx context.Context, userID, ref string) error {
	acc, err := s.Authorize(ctx, userID, ref, RoleAdmin)
	if err != nil {
		return err
	}
	if acc.Workspace.Kind == KindPersonal {
		return ErrPersonalWorkspace
	}
	return store.WithTx(ctx, s.db, func(q store.Querier) error {
		if err := lockWorkspace(ctx, q, acc.Workspace.ID); err != nil {
			return err
		}
		var sites int
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM sites WHERE workspace_id = $1", acc.Workspace.ID).Scan(&sites); err != nil {
			return fmt.Errorf("count sites: %w", err)
		}
		if sites > 0 {
			return ErrNotEmpty
		}
		if _, err := q.ExecContext(ctx, "DELETE FROM workspaces WHERE id = $1", acc.Workspace.ID); err != nil {
			return fmt.Errorf("delete workspace: %w", err)
		}
		return nil
	})
}
