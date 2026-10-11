package workspaces

import (
	"context"
	"fmt"
	"time"

	"github.com/zachlagden/openhere/internal/ids"
	"github.com/zachlagden/openhere/internal/slug"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/validate"
)

const (
	personalName       = "Personal"
	personalSlugPrefix = "personal-"
	personalSlugChars  = 12
	MaxNameLength      = 64
	derivedSlugLength  = 40
	slugAttempts       = 5
	slugSuffixLength   = 6
	fallbackSlug       = "workspace"
)

func PersonalSlug(userID string) string {
	n := min(len(userID), personalSlugChars)
	return personalSlugPrefix + userID[:n]
}

func CreatePersonal(ctx context.Context, q store.Querier, userID string, at time.Time) (Workspace, error) {
	ws := Workspace{
		ID: userID, Slug: PersonalSlug(userID), Name: personalName, Kind: KindPersonal,
		CreatedBy: userID, CreatedAt: at, UpdatedAt: at,
	}
	if err := insertWorkspace(ctx, q, ws); err != nil {
		return Workspace{}, err
	}
	if _, err := insertGrant(ctx, q, Grant{WorkspaceID: ws.ID, UserID: userID, Role: RoleAdmin, ScopeType: ScopeWorkspace}, at); err != nil {
		return Workspace{}, err
	}
	return ws, nil
}

func (s *Service) CreateShared(ctx context.Context, userID, name, requestedSlug string) (Access, error) {
	if err := s.allowSharedCreation(ctx, userID); err != nil {
		return Access{}, err
	}
	cleanName, err := validate.Line("Name", name, MaxNameLength)
	if err != nil {
		return Access{}, err
	}
	if requestedSlug != "" {
		return s.createSharedWithSlug(ctx, userID, cleanName, requestedSlug)
	}
	return s.createSharedDerived(ctx, userID, cleanName)
}

func (s *Service) createSharedWithSlug(ctx context.Context, userID, name, requested string) (Access, error) {
	if err := slug.Check(requested); err != nil {
		return Access{}, err
	}
	acc, err := s.insertShared(ctx, userID, name, requested)
	if store.IsUniqueViolation(err) {
		return Access{}, ErrSlugTaken
	}
	return acc, err
}

func (s *Service) createSharedDerived(ctx context.Context, userID, name string) (Access, error) {
	base := slug.From(name, derivedSlugLength)
	if base == "" || slug.Check(base) != nil {
		base = fallbackSlug
	}
	candidate := base
	for range slugAttempts {
		acc, err := s.insertShared(ctx, userID, name, candidate)
		if !store.IsUniqueViolation(err) {
			return acc, err
		}
		suffix, idErr := ids.New()
		if idErr != nil {
			return Access{}, idErr
		}
		candidate = base + "-" + suffix[:slugSuffixLength]
	}
	return Access{}, ErrSlugTaken
}

func (s *Service) insertShared(ctx context.Context, userID, name, workspaceSlug string) (Access, error) {
	id, err := ids.New()
	if err != nil {
		return Access{}, err
	}
	now := s.clock()
	ws := Workspace{ID: id, Slug: workspaceSlug, Name: name, Kind: KindShared, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	var grant Grant
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		if err := insertWorkspace(ctx, q, ws); err != nil {
			return err
		}
		var grantErr error
		grant, grantErr = insertGrant(ctx, q, Grant{WorkspaceID: id, UserID: userID, Role: RoleAdmin, ScopeType: ScopeWorkspace}, now)
		return grantErr
	})
	if err != nil {
		return Access{}, err
	}
	return Access{Workspace: ws, Grants: []Grant{grant}}, nil
}

func insertWorkspace(ctx context.Context, q store.Querier, ws Workspace) error {
	_, err := q.ExecContext(ctx,
		"INSERT INTO workspaces (id, slug, name, kind, created_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		ws.ID, ws.Slug, ws.Name, string(ws.Kind), ws.CreatedBy, ws.CreatedAt, ws.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert workspace: %w", err)
	}
	return nil
}
