package workspaces

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

func (s *Service) SharedByRef(ctx context.Context, ref string) (Workspace, error) {
	ws, err := scanWorkspace(s.db.QueryRowContext(ctx, "SELECT "+workspaceColumns+" FROM workspaces w WHERE w.id = $1 OR w.slug = $1", ref))
	if errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, fmt.Errorf("load workspace: %w", err)
	}
	if ws.Kind == KindPersonal {
		return Workspace{}, ErrPersonalWorkspace
	}
	return ws, nil
}

func AddWorkspaceGrant(ctx context.Context, q store.Querier, workspaceID, userID string, role Role, at time.Time) error {
	_, err := insertGrant(ctx, q, Grant{WorkspaceID: workspaceID, UserID: userID, Role: role, ScopeType: ScopeWorkspace}, at)
	return err
}
