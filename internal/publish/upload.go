package publish

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sitepath"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

func (s *Service) Upload(ctx context.Context, userID, siteRef, versionID, filePath string, body io.Reader) (FileSpec, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleWrite)
	if err != nil {
		return FileSpec{}, err
	}
	if err := sitepath.Validate(filePath); err != nil {
		return FileSpec{}, err
	}
	spec, err := s.uploadTarget(ctx, site.ID, versionID, filePath)
	if err != nil {
		return FileSpec{}, err
	}
	staged, err := s.files.Stage(body, spec.Size, spec.SHA256)
	if err != nil {
		return FileSpec{}, mapStorageError(err)
	}
	return spec, s.commitUpload(ctx, staged, site.ID, versionID, filePath)
}

func (s *Service) uploadTarget(ctx context.Context, siteID, versionID, filePath string) (FileSpec, error) {
	version, err := loadVersion(ctx, s.db, siteID, versionID)
	if err != nil {
		return FileSpec{}, err
	}
	if err := s.requireOpen(version); err != nil {
		return FileSpec{}, err
	}
	spec, err := loadFile(ctx, s.db, version.ID, filePath)
	if err != nil {
		return FileSpec{}, err
	}
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return FileSpec{}, err
	}
	if spec.Size > cfg.MaxFileBytes {
		return FileSpec{}, fmt.Errorf("%w: %q", ErrFileTooLarge, filePath)
	}
	return spec, nil
}

func (s *Service) requireOpen(version Version) error {
	if version.Status != StatusPending {
		return ErrVersionFinalized
	}
	if expired(version, s.clock()) {
		return ErrVersionExpired
	}
	return nil
}

func (s *Service) commitUpload(ctx context.Context, staged *sitefiles.Staged, siteID, versionID, filePath string) error {
	unlock := s.locks.rlock(versionID)
	defer unlock()
	version, err := loadVersion(ctx, s.db, siteID, versionID)
	if err == nil {
		err = s.requireOpen(version)
	}
	if err != nil {
		return errors.Join(err, staged.Discard())
	}
	return mapStorageError(staged.Commit(s.files, siteID, versionID, filePath))
}

func mapStorageError(err error) error {
	switch {
	case errors.Is(err, sitefiles.ErrSizeMismatch):
		return ErrSizeMismatch
	case errors.Is(err, sitefiles.ErrHashMismatch):
		return ErrHashMismatch
	default:
		return err
	}
}
