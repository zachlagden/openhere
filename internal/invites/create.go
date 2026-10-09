package invites

import (
	"context"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/ids"
	"github.com/zachlagden/selfhost/internal/store"
)

func (s *Service) Create(ctx context.Context, actor auth.User, in CreateInput) (Created, error) {
	target, err := s.authorize(ctx, actor, in.Workspace)
	if err != nil {
		return Created{}, err
	}
	email, err := auth.NormalizeEmail(in.Email)
	if err != nil {
		return Created{}, err
	}
	role, err := parseRole(target.ID, in.Role)
	if err != nil {
		return Created{}, err
	}
	if err := s.ensureNoAccount(ctx, email); err != nil {
		return Created{}, err
	}
	inv := Invite{
		Email: email, InvitedByID: actor.ID, InvitedByEmail: actor.Email,
		WorkspaceID: target.ID, WorkspaceSlug: target.Slug, WorkspaceName: target.Name, Role: role,
	}
	token, err := s.issue(ctx, &inv, "")
	if err != nil {
		return Created{}, err
	}
	return s.deliver(ctx, inv, token, in.SendEmail), nil
}

func (s *Service) ensureNoAccount(ctx context.Context, email string) error {
	_, err := s.auth.UserByEmail(ctx, email)
	if err == nil {
		return ErrEmailRegistered
	}
	if errors.Is(err, auth.ErrUserNotFound) {
		return nil
	}
	return err
}

func (s *Service) issue(ctx context.Context, inv *Invite, replacing string) (string, error) {
	id, err := ids.New()
	if err != nil {
		return "", err
	}
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	now := s.clock()
	inv.ID, inv.CreatedAt, inv.ExpiresAt = id, now, now.Add(Lifetime)
	inv.AcceptedAt, inv.RevokedAt = nil, nil
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		if replacing != "" {
			if err := markRevoked(ctx, q, replacing, now); err != nil {
				return err
			}
		}
		return insertInvite(ctx, q, *inv, auth.HashToken(token))
	})
	if err != nil {
		return "", fmt.Errorf("store invite: %w", err)
	}
	return token, nil
}

func (s *Service) deliver(ctx context.Context, inv Invite, token string, wantEmail bool) Created {
	created := Created{Invite: inv, Link: s.linkFor(token), EmailAvailable: s.mailer.Configured()}
	if !wantEmail || !created.EmailAvailable {
		return created
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := s.mailer.Send(sendCtx, s.message(inv, created.Link)); err != nil {
		s.logger.Warn("send invite email", "invite_id", inv.ID, "reason", reasonOf(err))
		created.EmailError = reasonOf(err)
		return created
	}
	created.EmailSent = true
	return created
}
