package invites

import (
	"context"
	"errors"
	"strings"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const maxDisplayName = 64

func (s *Service) pending(ctx context.Context, token string) (Invite, error) {
	if len(token) != auth.TokenLength {
		return Invite{}, ErrInviteInvalid
	}
	inv, err := loadBy(ctx, s.db, "i.token_hash = $1", auth.HashToken(token))
	if errors.Is(err, ErrInviteNotFound) {
		return Invite{}, ErrInviteInvalid
	}
	if err != nil {
		return Invite{}, err
	}
	if inv.Status(s.clock()) != StatusPending {
		return Invite{}, ErrInviteInvalid
	}
	return inv, nil
}

func (s *Service) Preview(ctx context.Context, token string) (Preview, error) {
	inv, err := s.pending(ctx, token)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Email: inv.Email, WorkspaceName: inv.WorkspaceName}, nil
}

func (s *Service) Accept(ctx context.Context, token, password, displayName string) (auth.User, error) {
	inv, err := s.pending(ctx, token)
	if err != nil {
		return auth.User{}, err
	}
	name, err := cleanDisplayName(displayName)
	if err != nil {
		return auth.User{}, err
	}
	creds, err := s.auth.PrepareCredentials(inv.Email, password)
	if err != nil {
		return auth.User{}, err
	}
	var created auth.User
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		var txErr error
		created, txErr = s.complete(ctx, q, inv, creds, name)
		return txErr
	})
	if store.IsUniqueViolation(err) {
		return auth.User{}, ErrInviteInvalid
	}
	if err != nil {
		return auth.User{}, err
	}
	return created, nil
}

func (s *Service) complete(ctx context.Context, q store.Querier, inv Invite, creds auth.Credentials, name string) (auth.User, error) {
	now := s.clock()
	claimed, err := q.ExecContext(ctx,
		"UPDATE invites SET accepted_at = $1 WHERE id = $2 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > $1", now, inv.ID)
	if err != nil {
		return auth.User{}, err
	}
	changed, err := claimed.RowsAffected()
	if err != nil {
		return auth.User{}, err
	}
	if changed != 1 {
		return auth.User{}, ErrInviteInvalid
	}
	user, err := s.auth.InsertMember(ctx, q, creds, name)
	if err != nil {
		return auth.User{}, err
	}
	if inv.WorkspaceID != "" {
		if err := workspaces.AddWorkspaceGrant(ctx, q, inv.WorkspaceID, user.ID, inv.Role, now); err != nil {
			return auth.User{}, err
		}
	}
	return user, nil
}

func cleanDisplayName(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return validate.Line("Display name", raw, maxDisplayName)
}
