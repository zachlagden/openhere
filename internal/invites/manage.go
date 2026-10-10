package invites

import (
	"context"
	"errors"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

func (s *Service) List(ctx context.Context, actor auth.User) ([]Invite, error) {
	if actor.IsRoot {
		return listWhere(ctx, s.db, "")
	}
	return listWhere(ctx, s.db, " WHERE i.invited_by = $1", actor.ID)
}

func (s *Service) visible(ctx context.Context, actor auth.User, id string) (Invite, error) {
	inv, err := loadBy(ctx, s.db, "i.id = $1", id)
	if err != nil {
		return Invite{}, err
	}
	if !actor.IsRoot && inv.InvitedByID != actor.ID {
		return Invite{}, ErrInviteNotFound
	}
	return inv, nil
}

func (s *Service) Revoke(ctx context.Context, actor auth.User, id string) error {
	inv, err := s.visible(ctx, actor, id)
	if err != nil {
		return err
	}
	if inv.AcceptedAt != nil {
		return ErrAccepted
	}
	return markRevoked(ctx, s.db, inv.ID, s.clock())
}

func (s *Service) Resend(ctx context.Context, actor auth.User, id string, sendEmail bool) (Created, error) {
	old, err := s.visible(ctx, actor, id)
	if err != nil {
		return Created{}, err
	}
	if old.AcceptedAt != nil {
		return Created{}, ErrAccepted
	}
	if err := s.reauthorize(ctx, actor, old); err != nil {
		return Created{}, err
	}
	if err := s.ensureNoAccount(ctx, old.Email); err != nil {
		return Created{}, err
	}
	fresh := old
	fresh.InvitedByID, fresh.InvitedByEmail = actor.ID, actor.Email
	token, err := s.issue(ctx, &fresh, old.ID)
	if err != nil {
		return Created{}, err
	}
	return s.deliver(ctx, fresh, token, sendEmail), nil
}

func (s *Service) reauthorize(ctx context.Context, actor auth.User, inv Invite) error {
	_, err := s.authorize(ctx, actor, inv.WorkspaceID)
	if errors.Is(err, workspaces.ErrNotFound) && inv.WorkspaceID != "" {
		return ErrNotPermitted
	}
	return err
}
