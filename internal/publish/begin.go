package publish

import (
	"context"
	"fmt"

	"github.com/zachlagden/selfhost/internal/ids"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type BeginInput struct {
	UserID      string
	Workspace   string
	Slug        string
	Folder      string
	DisplayName string
	Description string
	SpaMode     bool
	Files       []FileSpec
}

type NewVersionInput struct {
	UserID        string
	Site          string
	BaseVersionID string
	Files         []FileSpec
}

type Pending struct {
	Site    sites.Site
	Version Version
	Files   []FileSpec
}

func (s *Service) Begin(ctx context.Context, in BeginInput) (Pending, error) {
	acc, err := s.resolveWorkspace(ctx, in.UserID, in.Workspace)
	if err != nil {
		return Pending{}, err
	}
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return Pending{}, err
	}
	files, err := prepareManifest(in.Files, cfg)
	if err != nil {
		return Pending{}, err
	}
	folderID, err := s.resolveFolder(ctx, acc.Workspace.ID, in.Folder)
	if err != nil {
		return Pending{}, err
	}
	site, err := s.sites.CreateSite(ctx, sites.CreateInput{
		UserID: in.UserID, Workspace: acc.Workspace.ID, Slug: in.Slug, DisplayName: in.DisplayName,
		Description: in.Description, FolderID: folderID, SpaMode: in.SpaMode,
	})
	if err != nil {
		return Pending{}, err
	}
	version, err := s.createVersion(ctx, site, in.UserID, "", files)
	if err != nil {
		s.discardShell(ctx, in.UserID, site)
		return Pending{}, err
	}
	return Pending{Site: site, Version: version, Files: files}, nil
}

func (s *Service) discardShell(ctx context.Context, userID string, site sites.Site) {
	if err := s.sites.DeleteSite(ctx, userID, site.ID); err != nil {
		s.logger.Error("remove site after failed publish", "site_id", site.ID, "error", err)
	}
}

func (s *Service) resolveWorkspace(ctx context.Context, userID, ref string) (workspaces.Access, error) {
	if ref == "" {
		return s.ws.Personal(ctx, userID)
	}
	return s.ws.Access(ctx, userID, ref)
}

func (s *Service) resolveFolder(ctx context.Context, workspaceID, ref string) (string, error) {
	if ref == "" {
		return "", nil
	}
	folder, err := s.sites.FindFolder(ctx, workspaceID, ref)
	if err != nil {
		return "", err
	}
	return folder.ID, nil
}

func (s *Service) NewVersion(ctx context.Context, in NewVersionInput) (Pending, error) {
	site, err := s.sites.Authorize(ctx, in.UserID, in.Site, workspaces.RoleWrite)
	if err != nil {
		return Pending{}, err
	}
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return Pending{}, err
	}
	files, err := prepareManifest(in.Files, cfg)
	if err != nil {
		return Pending{}, err
	}
	if in.BaseVersionID != "" && in.BaseVersionID != site.CurrentVersionID {
		return Pending{}, &ConflictError{CurrentVersionID: site.CurrentVersionID}
	}
	version, err := s.createVersion(ctx, site, in.UserID, in.BaseVersionID, files)
	if err != nil {
		return Pending{}, err
	}
	return Pending{Site: site, Version: version, Files: files}, nil
}

func (s *Service) createVersion(ctx context.Context, site sites.Site, userID, base string, files []FileSpec) (Version, error) {
	id, err := ids.New()
	if err != nil {
		return Version{}, err
	}
	var total int64
	for _, file := range files {
		total += file.Size
	}
	version := Version{
		ID: id, SiteID: site.ID, Status: StatusPending, CreatedBy: userID, CreatedAt: s.clock(),
		FileCount: len(files), TotalBytes: total, BaseVersionID: base,
	}
	if err := insertVersion(ctx, s.db, version, files); err != nil {
		return Version{}, fmt.Errorf("create version: %w", err)
	}
	return version, nil
}
