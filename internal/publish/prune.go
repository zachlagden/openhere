package publish

import (
	"context"
	"fmt"
)

func (s *Service) prune(ctx context.Context, siteID string) {
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		s.logger.Error("prune versions: load settings", "site_id", siteID, "error", err)
		return
	}
	doomed, err := s.pruneCandidates(ctx, siteID, cfg.KeepVersions)
	if err != nil {
		s.logger.Error("prune versions", "site_id", siteID, "error", err)
		return
	}
	for _, versionID := range doomed {
		if err := s.removeVersion(ctx, siteID, versionID); err != nil {
			s.logger.Error("prune version", "site_id", siteID, "version_id", versionID, "error", err)
		}
	}
}

func (s *Service) pruneCandidates(ctx context.Context, siteID string, keep int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT v.id, CASE WHEN s.current_version_id = v.id THEN 1 ELSE 0 END
		FROM versions v JOIN sites s ON s.id = v.site_id
		WHERE v.site_id = $1 AND v.status = $2
		ORDER BY v.finalized_at DESC, v.created_at DESC, v.id DESC`, siteID, StatusFinalized)
	if err != nil {
		return nil, fmt.Errorf("list finalized versions: %w", err)
	}
	var ordered []string
	livePresent := false
	for rows.Next() {
		var id string
		var live int
		if err := rows.Scan(&id, &live); err != nil {
			return nil, closeRows(rows, fmt.Errorf("scan finalized version: %w", err))
		}
		if live == 1 {
			livePresent = true
			continue
		}
		ordered = append(ordered, id)
	}
	if err := rows.Err(); err != nil {
		return nil, closeRows(rows, fmt.Errorf("read finalized versions: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close finalized version rows: %w", err)
	}
	room := keep
	if livePresent {
		room--
	}
	return surplus(ordered, room), nil
}

func surplus(newestFirst []string, room int) []string {
	room = max(room, 0)
	if len(newestFirst) <= room {
		return nil
	}
	return newestFirst[room:]
}

func (s *Service) removeVersion(ctx context.Context, siteID, versionID string) error {
	unlock := s.locks.lock(versionID)
	defer unlock()
	removed, err := deleteVersionRow(ctx, s.db, siteID, versionID)
	if err != nil || !removed {
		return err
	}
	return s.files.RemoveVersion(siteID, versionID)
}
