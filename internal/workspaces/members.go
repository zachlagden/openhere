package workspaces

import (
	"context"
	"strings"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/validate"
)

type AddMemberInput struct {
	Email     string
	Role      string
	ScopeType string
	ScopeID   string
}

func (s *Service) ListMembers(ctx context.Context, actorID, ref string) ([]Member, error) {
	acc, err := s.Authorize(ctx, actorID, ref, RoleAdmin)
	if err != nil {
		return nil, err
	}
	return listMembers(ctx, s.db, acc.Workspace.ID)
}

func (s *Service) AddMember(ctx context.Context, actorID, ref string, in AddMemberInput) (Member, error) {
	acc, err := s.Authorize(ctx, actorID, ref, RoleAdmin)
	if err != nil {
		return Member{}, err
	}
	if acc.Workspace.Kind == KindPersonal {
		return Member{}, ErrPersonalWorkspace
	}
	grant, err := parseGrant(in)
	if err != nil {
		return Member{}, err
	}
	grant.WorkspaceID = acc.Workspace.ID
	var added Member
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		return s.addMember(ctx, q, in, grant, &added)
	})
	if store.IsUniqueViolation(err) {
		return Member{}, ErrMemberExists
	}
	return added, err
}

func (s *Service) addMember(ctx context.Context, q store.Querier, in AddMemberInput, grant Grant, out *Member) error {
	userID, err := userIDByEmail(ctx, q, in.Email)
	if err != nil {
		return err
	}
	grant.UserID = userID
	if err := resolveScope(ctx, q, &grant, in.ScopeID); err != nil {
		return err
	}
	created, err := insertGrant(ctx, q, grant, s.clock())
	if err != nil {
		return err
	}
	member, err := memberByID(ctx, q, grant.WorkspaceID, created.ID)
	if err != nil {
		return err
	}
	*out = member
	return nil
}

func (s *Service) UpdateMember(ctx context.Context, actorID, ref, memberID, role string) (Member, error) {
	acc, err := s.Authorize(ctx, actorID, ref, RoleAdmin)
	if err != nil {
		return Member{}, err
	}
	if acc.Workspace.Kind == KindPersonal {
		return Member{}, ErrPersonalWorkspace
	}
	newRole, ok := ParseRole(role)
	if !ok {
		return Member{}, validate.Invalid("Role must be admin, write or read.")
	}
	var updated Member
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		return changeRole(ctx, q, acc.Workspace.ID, memberID, newRole, &updated)
	})
	return updated, err
}

func changeRole(ctx context.Context, q store.Querier, workspaceID, memberID string, role Role, out *Member) error {
	if err := lockWorkspace(ctx, q, workspaceID); err != nil {
		return err
	}
	current, err := memberByID(ctx, q, workspaceID, memberID)
	if err != nil {
		return err
	}
	if current.ScopeType != ScopeWorkspace && role == RoleAdmin {
		return validate.Invalid("Admin applies to the whole workspace only.")
	}
	if current.Role == RoleAdmin && role != RoleAdmin {
		if err := ensureAnotherAdmin(ctx, q, workspaceID, current); err != nil {
			return err
		}
	}
	if err := updateGrantRole(ctx, q, memberID, role); err != nil {
		return err
	}
	current.Role = role
	*out = current
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, actorID, ref, memberID string) error {
	acc, err := s.Authorize(ctx, actorID, ref, RoleAdmin)
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
		current, err := memberByID(ctx, q, acc.Workspace.ID, memberID)
		if err != nil {
			return err
		}
		if current.Role == RoleAdmin {
			if err := ensureAnotherAdmin(ctx, q, acc.Workspace.ID, current); err != nil {
				return err
			}
		}
		return deleteGrant(ctx, q, memberID)
	})
}

func parseGrant(in AddMemberInput) (Grant, error) {
	role, ok := ParseRole(in.Role)
	if !ok {
		return Grant{}, validate.Invalid("Role must be admin, write or read.")
	}
	scope := ScopeType(strings.TrimSpace(in.ScopeType))
	if scope == "" {
		scope = ScopeWorkspace
	}
	switch scope {
	case ScopeWorkspace:
		if in.ScopeID != "" {
			return Grant{}, validate.Invalid("A workspace-wide grant takes no scope id.")
		}
	case ScopeFolder, ScopeSite:
		if in.ScopeID == "" {
			return Grant{}, validate.Invalid("A folder or site grant needs a scope id.")
		}
		if role == RoleAdmin {
			return Grant{}, validate.Invalid("Admin applies to the whole workspace only.")
		}
	default:
		return Grant{}, validate.Invalid("Scope type must be workspace, folder or site.")
	}
	return Grant{Role: role, ScopeType: scope}, nil
}
