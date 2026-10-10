package sites

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zachlagden/selfhost/internal/ids"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type CreateInput struct {
	UserID      string
	Workspace   string
	Slug        string
	DisplayName string
	Description string
	FolderID    string
	SpaMode     bool
}

type FolderChange struct {
	Set bool
	ID  string
}

type UpdateInput struct {
	DisplayName *string
	Description *string
	SpaMode     *bool
	Analytics   *bool
	Folder      FolderChange
}

func (s *Service) CreateSite(ctx context.Context, in CreateInput) (Site, error) {
	acc, err := s.ws.Access(ctx, in.UserID, in.Workspace)
	if err != nil {
		return Site{}, err
	}
	if err := s.requireWriteIn(ctx, acc, in.FolderID); err != nil {
		return Site{}, err
	}
	site, err := s.buildSite(in, acc.Workspace)
	if err != nil {
		return Site{}, err
	}
	if site.AccessMode, err = s.newSiteAccessMode(ctx); err != nil {
		return Site{}, err
	}
	if site.AnalyticsEnabled, err = s.newSiteAnalytics(ctx); err != nil {
		return Site{}, err
	}
	if in.Slug != "" {
		if err := insertSite(ctx, s.db, site); err != nil {
			return Site{}, mapSlugError(err)
		}
	} else if site, err = s.insertWithGeneratedSlug(ctx, site); err != nil {
		return Site{}, err
	}
	site.Role = acc.SiteRole(site.ID, site.FolderID)
	s.changed(site.Slug)
	return site, nil
}

func (s *Service) buildSite(in CreateInput, ws workspaces.Workspace) (Site, error) {
	if in.Slug != "" {
		if err := CheckSlug(in.Slug); err != nil {
			return Site{}, err
		}
	}
	description, err := validate.Text("Description", in.Description, MaxDescriptionLength)
	if err != nil {
		return Site{}, err
	}
	name := in.DisplayName
	if name == "" {
		name = in.Slug
	}
	var displayName string
	if name != "" {
		if displayName, err = validate.Line("Display name", name, MaxDisplayNameLength); err != nil {
			return Site{}, err
		}
	}
	id, err := ids.New()
	if err != nil {
		return Site{}, err
	}
	now := s.clock()
	return Site{
		ID: id, WorkspaceID: ws.ID, WorkspaceSlug: ws.Slug, FolderID: in.FolderID, Slug: in.Slug, DisplayName: displayName,
		Description: description, CreatedBy: in.UserID, SpaMode: in.SpaMode, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Service) insertWithGeneratedSlug(ctx context.Context, site Site) (Site, error) {
	for range slugAttempts {
		generated, err := s.generate()
		if err != nil {
			return Site{}, err
		}
		site.Slug = generated
		if site.DisplayName == "" {
			site.DisplayName = generated
		}
		err = insertSite(ctx, s.db, site)
		if err == nil {
			return site, nil
		}
		if !store.IsUniqueViolation(err) {
			return Site{}, err
		}
	}
	return Site{}, ErrSlugGeneration
}

func mapSlugError(err error) error {
	if store.IsUniqueViolation(err) {
		return ErrSlugTaken
	}
	return err
}

func (s *Service) requireWriteIn(ctx context.Context, acc workspaces.Access, folderID string) error {
	if folderID == "" {
		return workspaces.Require(acc.WorkspaceRole(), workspaces.RoleWrite)
	}
	folder, err := loadFolder(ctx, s.db, acc.Workspace.ID, folderID)
	if err != nil {
		return err
	}
	return workspaces.Require(acc.FolderRole(folder.ID), workspaces.RoleWrite)
}

func (s *Service) GetSite(ctx context.Context, userID, ref string) (Site, error) {
	site, _, err := s.siteAccess(ctx, userID, ref, workspaces.RoleRead)
	return site, err
}

func (s *Service) UpdateSite(ctx context.Context, userID, ref string, in UpdateInput) (Site, error) {
	site, acc, err := s.siteAccess(ctx, userID, ref, workspaces.RoleWrite)
	if err != nil {
		return Site{}, err
	}
	if err := applyFieldChanges(&site, in); err != nil {
		return Site{}, err
	}
	if in.Folder.Set && in.Folder.ID != site.FolderID {
		if err := s.requireWriteIn(ctx, acc, in.Folder.ID); err != nil {
			return Site{}, err
		}
		site.FolderID = in.Folder.ID
	}
	site.UpdatedAt = s.clock()
	if err := updateSite(ctx, s.db, site); err != nil {
		return Site{}, err
	}
	site.Role = acc.SiteRole(site.ID, site.FolderID)
	s.changed(site.Slug)
	return site, nil
}

func applyFieldChanges(site *Site, in UpdateInput) error {
	if in.DisplayName != nil {
		name, err := validate.Line("Display name", *in.DisplayName, MaxDisplayNameLength)
		if err != nil {
			return err
		}
		site.DisplayName = name
	}
	if in.SpaMode != nil {
		site.SpaMode = *in.SpaMode
	}
	if in.Analytics != nil {
		site.AnalyticsEnabled = *in.Analytics
	}
	if in.Description != nil {
		description, err := validate.Text("Description", *in.Description, MaxDescriptionLength)
		if err != nil {
			return err
		}
		site.Description = description
	}
	return nil
}

func updateSite(ctx context.Context, q store.Querier, site Site) error {
	var folderID sql.NullString
	if site.FolderID != "" {
		folderID = sql.NullString{String: site.FolderID, Valid: true}
	}
	_, err := q.ExecContext(ctx,
		"UPDATE sites SET display_name = $1, description = $2, folder_id = $3, spa_mode = $4, analytics_enabled = $5, updated_at = $6 WHERE id = $7",
		site.DisplayName, site.Description, folderID, site.SpaMode, site.AnalyticsEnabled, site.UpdatedAt, site.ID)
	if err != nil {
		return fmt.Errorf("update site: %w", err)
	}
	return nil
}

func (s *Service) DeleteSite(ctx context.Context, userID, ref string) error {
	site, _, err := s.siteAccess(ctx, userID, ref, workspaces.RoleWrite)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sites WHERE id = $1", site.ID); err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	s.changed(site.Slug)
	s.removeSiteFiles(site.ID)
	return nil
}

func (s *Service) removeSiteFiles(siteID string) {
	if s.files == nil {
		return
	}
	if err := s.files.RemoveSite(siteID); err != nil {
		s.logger.Error("remove site files", "site_id", siteID, "error", err)
	}
}

func (s *Service) Authorize(ctx context.Context, userID, ref string, need workspaces.Role) (Site, error) {
	site, _, err := s.siteAccess(ctx, userID, ref, need)
	return site, err
}

func (s *Service) siteAccess(ctx context.Context, userID, ref string, need workspaces.Role) (Site, workspaces.Access, error) {
	site, err := loadSite(ctx, s.db, ref)
	if err != nil {
		return Site{}, workspaces.Access{}, err
	}
	acc, err := s.ws.AccessByID(ctx, userID, site.WorkspaceID)
	if err != nil {
		return Site{}, workspaces.Access{}, err
	}
	site.Role = acc.SiteRole(site.ID, site.FolderID)
	if err := workspaces.Require(site.Role, need); err != nil {
		return Site{}, workspaces.Access{}, err
	}
	return site, acc, nil
}
