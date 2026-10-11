package workspaces

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/zachlagden/openhere/internal/store"
)

const memberSelect = `SELECT a.id, a.workspace_id, a.user_id, a.role, a.scope_type, a.folder_id, a.site_id, a.created_at,
	u.email, f.name, st.slug
	FROM workspace_access a
	JOIN users u ON u.id = a.user_id
	LEFT JOIN folders f ON f.id = a.folder_id
	LEFT JOIN sites st ON st.id = a.site_id`

func scanMember(row scanner) (Member, error) {
	var m Member
	var folderID, siteID, folderName, siteSlug sql.NullString
	err := row.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.Role, &m.ScopeType, &folderID, &siteID, &m.CreatedAt,
		&m.Email, &folderName, &siteSlug)
	if err != nil {
		return Member{}, err
	}
	m.FolderID = folderID.String
	m.SiteID = siteID.String
	m.ScopeName = folderName.String + siteSlug.String
	return m, nil
}

func listMembers(ctx context.Context, q store.Querier, workspaceID string) ([]Member, error) {
	rows, err := q.QueryContext(ctx, memberSelect+" WHERE a.workspace_id = $1 ORDER BY u.email, a.scope_type, a.created_at, a.id", workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	members := []Member{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("scan member: %w", err), rows.Close())
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read members: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close member rows: %w", err)
	}
	return members, nil
}

func memberByID(ctx context.Context, q store.Querier, workspaceID, memberID string) (Member, error) {
	m, err := scanMember(q.QueryRowContext(ctx, memberSelect+" WHERE a.workspace_id = $1 AND a.id = $2", workspaceID, memberID))
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	if err != nil {
		return Member{}, fmt.Errorf("load member: %w", err)
	}
	return m, nil
}

func userIDByEmail(ctx context.Context, q store.Querier, email string) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, "SELECT id FROM users WHERE email = $1", strings.ToLower(strings.TrimSpace(email))).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("look up user: %w", err)
	}
	return id, nil
}

func resolveScope(ctx context.Context, q store.Querier, g *Grant, scopeID string) error {
	var query string
	switch g.ScopeType {
	case ScopeFolder:
		query = "SELECT id FROM folders WHERE workspace_id = $1 AND id = $2"
	case ScopeSite:
		query = "SELECT id FROM sites WHERE workspace_id = $1 AND (id = $2 OR slug = $2)"
	default:
		return nil
	}
	var id string
	err := q.QueryRowContext(ctx, query, g.WorkspaceID, scopeID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("resolve scope: %w", err)
	}
	if g.ScopeType == ScopeFolder {
		g.FolderID = id
	} else {
		g.SiteID = id
	}
	return nil
}

func ensureAnotherAdmin(ctx context.Context, q store.Querier, workspaceID string, leaving Member) error {
	if leaving.ScopeType != ScopeWorkspace {
		return nil
	}
	var others int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workspace_access WHERE workspace_id = $1 AND scope_type = 'workspace' AND role = 'admin' AND id <> $2",
		workspaceID, leaving.ID).Scan(&others)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if others == 0 {
		return ErrLastAdmin
	}
	return nil
}

func updateGrantRole(ctx context.Context, q store.Querier, memberID string, role Role) error {
	if _, err := q.ExecContext(ctx, "UPDATE workspace_access SET role = $1 WHERE id = $2", string(role), memberID); err != nil {
		return fmt.Errorf("update grant: %w", err)
	}
	return nil
}

func deleteGrant(ctx context.Context, q store.Querier, memberID string) error {
	if _, err := q.ExecContext(ctx, "DELETE FROM workspace_access WHERE id = $1", memberID); err != nil {
		return fmt.Errorf("delete grant: %w", err)
	}
	return nil
}
