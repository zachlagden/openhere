package publish

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const temporaryFileTTL = 24 * time.Hour

func (s *Service) Maintain(ctx context.Context) error {
	var problems []error
	expired, err := s.expirePending(ctx)
	if err != nil {
		problems = append(problems, err)
	}
	if expired > 0 {
		s.logger.Info("removed expired pending versions", "count", expired)
	}
	orphans, err := s.sweepOrphans(ctx)
	if err != nil {
		problems = append(problems, err)
	}
	if orphans > 0 {
		s.logger.Info("removed orphaned site directories", "count", orphans)
	}
	stale, err := s.files.SweepTemporary(s.clock().Add(-temporaryFileTTL))
	if err != nil {
		problems = append(problems, err)
	}
	if stale > 0 {
		s.logger.Info("removed stale upload files", "count", stale)
	}
	return errors.Join(problems...)
}

type versionRef struct {
	siteID    string
	versionID string
}

func (s *Service) expirePending(ctx context.Context) (int, error) {
	refs, err := s.expiredRefs(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	var problems []error
	for _, ref := range refs {
		done, err := s.removePending(ctx, ref)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if done {
			removed++
		}
	}
	return removed, errors.Join(problems...)
}

func (s *Service) expiredRefs(ctx context.Context) ([]versionRef, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT site_id, id FROM versions WHERE status = $1 AND created_at < $2", StatusPending, s.clock().Add(-PendingTTL))
	if err != nil {
		return nil, fmt.Errorf("list expired versions: %w", err)
	}
	var refs []versionRef
	for rows.Next() {
		var ref versionRef
		if err := rows.Scan(&ref.siteID, &ref.versionID); err != nil {
			return nil, closeRows(rows, fmt.Errorf("scan expired version: %w", err))
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, closeRows(rows, fmt.Errorf("read expired versions: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close expired version rows: %w", err)
	}
	return refs, nil
}

func (s *Service) removePending(ctx context.Context, ref versionRef) (bool, error) {
	unlock := s.locks.lock(ref.versionID)
	defer unlock()
	result, err := s.db.ExecContext(ctx, "DELETE FROM versions WHERE id = $1 AND status = $2", ref.versionID, StatusPending)
	if err != nil {
		return false, fmt.Errorf("delete expired version: %w", err)
	}
	changed, err := rowsChanged(result, "delete expired version")
	if err != nil || !changed {
		return false, err
	}
	return true, s.files.RemoveVersion(ref.siteID, ref.versionID)
}

func (s *Service) sweepOrphans(ctx context.Context) (int, error) {
	siteIDs, err := s.files.SiteDirs()
	if err != nil {
		return 0, err
	}
	removed := 0
	var problems []error
	for _, siteID := range siteIDs {
		count, err := s.sweepSite(ctx, siteID)
		removed += count
		if err != nil {
			problems = append(problems, err)
		}
	}
	return removed, errors.Join(problems...)
}

func (s *Service) sweepSite(ctx context.Context, siteID string) (int, error) {
	versionIDs, err := s.files.VersionDirs(siteID)
	if err != nil {
		return 0, err
	}
	known, err := s.exists(ctx, "SELECT 1 FROM sites WHERE id = $1", siteID)
	if err != nil {
		return 0, err
	}
	if !known {
		return 1, s.files.RemoveSite(siteID)
	}
	removed := 0
	var problems []error
	for _, versionID := range versionIDs {
		present, err := s.exists(ctx, "SELECT 1 FROM versions WHERE id = $1 AND site_id = $2", versionID, siteID)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if present {
			continue
		}
		if err := s.files.RemoveVersion(siteID, versionID); err != nil {
			problems = append(problems, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(problems...)
}

func (s *Service) exists(ctx context.Context, query string, args ...any) (bool, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("check existence: %w", err)
	}
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, closeRows(rows, fmt.Errorf("check existence: %w", err))
	}
	if err := rows.Close(); err != nil {
		return false, fmt.Errorf("close existence rows: %w", err)
	}
	return found, nil
}
