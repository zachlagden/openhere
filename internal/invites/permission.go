package invites

import (
	"context"
	"fmt"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

func (s *Service) authorize(ctx context.Context, actor auth.User, workspaceRef string) (workspaces.Workspace, error) {
	if actor.IsRoot {
		return s.rootTarget(ctx, workspaceRef)
	}
	current, err := s.settings.Get(ctx)
	if err != nil {
		return workspaces.Workspace{}, fmt.Errorf("read invite policy: %w", err)
	}
	if current.WhoCanInvite != settings.InviteWorkspaceAdmins || workspaceRef == "" {
		return workspaces.Workspace{}, ErrNotPermitted
	}
	acc, err := s.workspaces.Authorize(ctx, actor.ID, workspaceRef, workspaces.RoleAdmin)
	if err != nil {
		return workspaces.Workspace{}, err
	}
	if acc.Workspace.Kind == workspaces.KindPersonal {
		return workspaces.Workspace{}, workspaces.ErrPersonalWorkspace
	}
	return acc.Workspace, nil
}

func (s *Service) rootTarget(ctx context.Context, workspaceRef string) (workspaces.Workspace, error) {
	if workspaceRef == "" {
		return workspaces.Workspace{}, nil
	}
	return s.workspaces.SharedByRef(ctx, workspaceRef)
}

func parseRole(workspaceID, raw string) (workspaces.Role, error) {
	if workspaceID == "" {
		if raw != "" {
			return workspaces.RoleNone, validate.Invalid("A role needs a workspace to apply to.")
		}
		return workspaces.RoleNone, nil
	}
	if raw == "" {
		return workspaces.RoleRead, nil
	}
	role, ok := workspaces.ParseRole(raw)
	if !ok {
		return workspaces.RoleNone, validate.Invalid("Role must be admin, write or read.")
	}
	return role, nil
}
