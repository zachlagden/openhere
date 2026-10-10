package invites

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const inviteSelect = `SELECT i.id, i.email, i.invited_by, u.email, i.workspace_id, w.slug, w.name, i.role,
	i.created_at, i.expires_at, i.accepted_at, i.revoked_at
	FROM invites i
	JOIN users u ON u.id = i.invited_by
	LEFT JOIN workspaces w ON w.id = i.workspace_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInvite(row rowScanner) (Invite, error) {
	var inv Invite
	var workspaceID, slug, name, role sql.NullString
	var accepted, revoked sql.NullTime
	err := row.Scan(&inv.ID, &inv.Email, &inv.InvitedByID, &inv.InvitedByEmail, &workspaceID, &slug, &name, &role,
		&inv.CreatedAt, &inv.ExpiresAt, &accepted, &revoked)
	if err != nil {
		return Invite{}, err
	}
	inv.WorkspaceID, inv.WorkspaceSlug, inv.WorkspaceName = workspaceID.String, slug.String, name.String
	inv.Role = workspaces.Role(role.String)
	if accepted.Valid {
		inv.AcceptedAt = &accepted.Time
	}
	if revoked.Valid {
		inv.RevokedAt = &revoked.Time
	}
	return inv, nil
}

func loadBy(ctx context.Context, q store.Querier, condition, arg string) (Invite, error) {
	inv, err := scanInvite(q.QueryRowContext(ctx, inviteSelect+" WHERE "+condition, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return Invite{}, ErrInviteNotFound
	}
	if err != nil {
		return Invite{}, fmt.Errorf("load invite: %w", err)
	}
	return inv, nil
}

func insertInvite(ctx context.Context, q store.Querier, inv Invite, tokenHash string) error {
	var workspaceID, role any
	if inv.WorkspaceID != "" {
		workspaceID, role = inv.WorkspaceID, string(inv.Role)
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO invites (id, email, token_hash, invited_by, workspace_id, role, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		inv.ID, inv.Email, tokenHash, inv.InvitedByID, workspaceID, role, inv.CreatedAt, inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert invite: %w", err)
	}
	return nil
}

func markRevoked(ctx context.Context, q store.Querier, id string, at any) error {
	_, err := q.ExecContext(ctx, "UPDATE invites SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL AND accepted_at IS NULL", at, id)
	if err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	return nil
}

func listWhere(ctx context.Context, q store.Querier, condition string, args ...any) ([]Invite, error) {
	rows, err := q.QueryContext(ctx, inviteSelect+condition+fmt.Sprintf(" ORDER BY i.created_at DESC, i.id LIMIT %d", maxListed), args...)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	list := []Invite{}
	for rows.Next() {
		inv, scanErr := scanInvite(rows)
		if scanErr != nil {
			return nil, errors.Join(fmt.Errorf("scan invite: %w", scanErr), rows.Close())
		}
		list = append(list, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read invites: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close invite rows: %w", err)
	}
	return list, nil
}
