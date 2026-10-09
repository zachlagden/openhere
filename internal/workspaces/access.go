package workspaces

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/store"
)

type Access struct {
	Workspace Workspace
	Grants    []Grant
}

func (a Access) RoleOn(res Resource) Role {
	res.WorkspaceID = a.Workspace.ID
	return EffectiveRole(a.Grants, res)
}

func (a Access) WorkspaceRole() Role {
	return a.RoleOn(WorkspaceResource(a.Workspace.ID))
}

func (a Access) FolderRole(folderID string) Role {
	return a.RoleOn(FolderResource(a.Workspace.ID, folderID))
}

func (a Access) SiteRole(siteID, folderID string) Role {
	return a.RoleOn(SiteResource(a.Workspace.ID, siteID, folderID))
}

func (a Access) ScopedOnly() bool {
	for _, g := range a.Grants {
		if g.ScopeType == ScopeWorkspace {
			return false
		}
	}
	return true
}

func (s *Service) Access(ctx context.Context, userID, ref string) (Access, error) {
	return loadAccess(ctx, s.db, userID, "w.id = $1 OR w.slug = $1", ref)
}

func (s *Service) AccessByID(ctx context.Context, userID, workspaceID string) (Access, error) {
	return loadAccess(ctx, s.db, userID, "w.id = $1", workspaceID)
}

func (s *Service) Authorize(ctx context.Context, userID, ref string, need Role) (Access, error) {
	acc, err := s.Access(ctx, userID, ref)
	if err != nil {
		return Access{}, err
	}
	if err := Require(acc.WorkspaceRole(), need); err != nil {
		return Access{}, err
	}
	return acc, nil
}

func loadAccess(ctx context.Context, q store.Querier, userID, condition, arg string) (Access, error) {
	ws, err := scanWorkspace(q.QueryRowContext(ctx, "SELECT "+workspaceColumns+" FROM workspaces w WHERE "+condition, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return Access{}, ErrNotFound
	}
	if err != nil {
		return Access{}, fmt.Errorf("load workspace: %w", err)
	}
	grants, err := loadGrants(ctx, q, userID, ws.ID)
	if err != nil {
		return Access{}, err
	}
	if len(grants) == 0 {
		return Access{}, ErrNotFound
	}
	return Access{Workspace: ws, Grants: grants}, nil
}

func loadGrants(ctx context.Context, q store.Querier, userID, workspaceID string) ([]Grant, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+grantColumns+" FROM workspace_access WHERE user_id = $1 AND workspace_id = $2", userID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("load grants: %w", err)
	}
	return collectGrants(rows)
}

func collectGrants(rows *sql.Rows) ([]Grant, error) {
	grants := []Grant{}
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("scan grant: %w", err), rows.Close())
		}
		grants = append(grants, g)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read grants: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close grant rows: %w", err)
	}
	return grants, nil
}
