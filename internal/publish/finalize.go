package publish

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type Result struct {
	Site    sites.Site
	Version Version
}

func (s *Service) Finalize(ctx context.Context, userID, siteRef, versionID string) (Result, error) {
	site, err := s.sites.Authorize(ctx, userID, siteRef, workspaces.RoleWrite)
	if err != nil {
		return Result{}, err
	}
	version, err := loadVersion(ctx, s.db, site.ID, versionID)
	if err != nil {
		return Result{}, err
	}
	if version.Status == StatusFinalized {
		return s.result(ctx, userID, site.ID, version.ID)
	}
	err = s.finalizePending(ctx, site, version)
	s.changed(site.Slug)
	if err != nil {
		return Result{}, err
	}
	s.prune(ctx, site.ID)
	return s.result(ctx, userID, site.ID, version.ID)
}

func (s *Service) result(ctx context.Context, userID, siteID, versionID string) (Result, error) {
	site, err := s.sites.Authorize(ctx, userID, siteID, workspaces.RoleRead)
	if err != nil {
		return Result{}, err
	}
	version, err := loadVersion(ctx, s.db, site.ID, versionID)
	if err != nil {
		return Result{}, err
	}
	version.Live = site.CurrentVersionID == version.ID
	return Result{Site: site, Version: version}, nil
}

func (s *Service) finalizePending(ctx context.Context, site sites.Site, version Version) error {
	unlock := s.locks.lock(version.ID)
	defer unlock()
	current, err := loadVersion(ctx, s.db, site.ID, version.ID)
	if err != nil {
		return err
	}
	if current.Status == StatusFinalized {
		return nil
	}
	if expired(current, s.clock()) {
		return ErrVersionExpired
	}
	files, err := loadFiles(ctx, s.db, current.ID)
	if err != nil {
		return err
	}
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	if err := checkLimits(files, cfg); err != nil {
		return err
	}
	if err := s.verifyFiles(site.ID, current.ID, files); err != nil {
		return err
	}
	err = store.WithTx(ctx, s.db, func(q store.Querier) error { return s.flip(ctx, q, site.ID, current) })
	if errors.Is(err, errNotPending) {
		return nil
	}
	return err
}

func (s *Service) verifyFiles(siteID, versionID string, files []FileSpec) error {
	var missing []string
	for _, file := range files {
		err := s.files.Verify(siteID, versionID, sitefiles.Expected{Path: file.Path, Size: file.Size, SHA256: file.SHA256})
		switch {
		case err == nil:
		case isFileProblem(err):
			missing = append(missing, file.Path)
		default:
			return fmt.Errorf("verify %q: %w", file.Path, err)
		}
	}
	if len(missing) > 0 {
		return &MissingFilesError{Paths: missing}
	}
	return nil
}

func isFileProblem(err error) bool {
	return errors.Is(err, sitefiles.ErrNotFound) || errors.Is(err, sitefiles.ErrNotRegular) ||
		errors.Is(err, sitefiles.ErrSizeMismatch) || errors.Is(err, sitefiles.ErrHashMismatch)
}

func (s *Service) flip(ctx context.Context, q store.Querier, siteID string, version Version) error {
	now := s.clock()
	result, err := q.ExecContext(ctx,
		"UPDATE versions SET status = $1, finalized_at = $2 WHERE id = $3 AND site_id = $4 AND status = $5",
		StatusFinalized, now, version.ID, siteID, StatusPending)
	if err != nil {
		return fmt.Errorf("finalize version: %w", err)
	}
	changed, err := rowsChanged(result, "finalize version")
	if err != nil {
		return err
	}
	if !changed {
		return errNotPending
	}
	return pointSiteAt(ctx, q, siteID, version.ID, version.BaseVersionID, now)
}

func pointSiteAt(ctx context.Context, q store.Querier, siteID, versionID, base string, now time.Time) error {
	query := "UPDATE sites SET current_version_id = $1, updated_at = $2 WHERE id = $3"
	args := []any{versionID, now, siteID}
	if base != "" {
		query += " AND current_version_id = $4"
		args = append(args, base)
	}
	result, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("point site at version: %w", err)
	}
	changed, err := rowsChanged(result, "point site at version")
	if err != nil {
		return err
	}
	if changed {
		return nil
	}
	return conflictFor(ctx, q, siteID)
}

func conflictFor(ctx context.Context, q store.Querier, siteID string) error {
	var current sql.NullString
	err := q.QueryRowContext(ctx, "SELECT current_version_id FROM sites WHERE id = $1", siteID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return workspaces.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read live version: %w", err)
	}
	return &ConflictError{CurrentVersionID: current.String}
}
