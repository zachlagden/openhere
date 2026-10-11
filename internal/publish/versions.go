package publish

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/zachlagden/openhere/internal/sitepath"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func (s *Service) List(ctx context.Context, userID, siteRef string) ([]Version, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleRead)
	if err != nil {
		return nil, err
	}
	versions, err := listVersions(ctx, s.db, site.ID, maxListedRows)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		versions[i].Live = versions[i].ID == site.CurrentVersionID
	}
	return versions, nil
}

func (s *Service) Restore(ctx context.Context, userID, siteRef, versionID string) (Result, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleWrite)
	if err != nil {
		return Result{}, err
	}
	version, err := loadVersion(ctx, s.db, site.ID, versionID)
	if err != nil {
		return Result{}, err
	}
	if version.Status != StatusFinalized {
		return Result{}, ErrNotFinalized
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE sites SET current_version_id = $1, updated_at = $2 WHERE id = $3
		AND EXISTS (SELECT 1 FROM versions WHERE id = $1 AND site_id = $3 AND status = $4)`,
		version.ID, s.clock(), site.ID, StatusFinalized)
	if err != nil {
		return Result{}, fmt.Errorf("restore version: %w", err)
	}
	changed, err := rowsChanged(result, "restore version")
	if err != nil {
		return Result{}, err
	}
	s.changed(site.Slug)
	if !changed {
		return Result{}, workspaces.ErrNotFound
	}
	return s.result(ctx, userID, site.ID, version.ID)
}

func (s *Service) DeleteVersion(ctx context.Context, userID, siteRef, versionID string) error {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleWrite)
	if err != nil {
		return err
	}
	version, err := loadVersion(ctx, s.db, site.ID, versionID)
	if err != nil {
		return err
	}
	if site.CurrentVersionID == version.ID {
		return ErrVersionLive
	}
	unlock := s.locks.lock(version.ID)
	defer unlock()
	removed, err := deleteVersionRow(ctx, s.db, site.ID, version.ID)
	if err != nil {
		return err
	}
	if !removed {
		return ErrVersionLive
	}
	return s.files.RemoveVersion(site.ID, version.ID)
}

func (s *Service) LiveManifest(ctx context.Context, userID, siteRef string) (Manifest, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleRead)
	if err != nil {
		return Manifest{}, err
	}
	if site.CurrentVersionID == "" {
		return Manifest{Files: []FileSpec{}}, nil
	}
	files, err := loadFiles(ctx, s.db, site.CurrentVersionID)
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{VersionID: site.CurrentVersionID, Files: files}, nil
}

type LiveFile struct {
	Spec FileSpec
	File *os.File
}

func (s *Service) OpenLiveFile(ctx context.Context, userID, siteRef, filePath string) (LiveFile, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleRead)
	if err != nil {
		return LiveFile{}, err
	}
	if err := sitepath.Validate(filePath); err != nil || site.CurrentVersionID == "" {
		return LiveFile{}, workspaces.ErrNotFound
	}
	spec, err := loadFile(ctx, s.db, site.CurrentVersionID, filePath)
	if errors.Is(err, ErrFileNotInList) {
		return LiveFile{}, workspaces.ErrNotFound
	}
	if err != nil {
		return LiveFile{}, err
	}
	file, _, err := s.files.Open(site.ID, site.CurrentVersionID, filePath)
	if err != nil {
		return LiveFile{}, err
	}
	return LiveFile{Spec: spec, File: file}, nil
}
