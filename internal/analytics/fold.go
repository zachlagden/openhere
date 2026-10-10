package analytics

import (
	"context"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/store"
)

func (r *Recorder) foldPaths(ctx context.Context, events []Event) []Event {
	known := map[dayKey]map[string]bool{}
	for i := range events {
		event := &events[i]
		if event.Path == PathNotFound || event.Path == PathOther {
			continue
		}
		key := dayKey{siteID: event.SiteID, day: event.Day()}
		paths, loaded := known[key]
		if !loaded {
			stored, err := storedPaths(ctx, r.db, key, r.maxPaths)
			if err != nil {
				r.logger.Warn("load the stored analytics paths", "error", err)
				stored = map[string]bool{}
			}
			paths = stored
			known[key] = paths
		}
		if paths[event.Path] {
			continue
		}
		if len(paths) >= r.maxPaths {
			event.Path = PathOther
			continue
		}
		paths[event.Path] = true
	}
	return events
}

func storedPaths(ctx context.Context, q store.Querier, key dayKey, limit int) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT DISTINCT path FROM analytics_events WHERE site_id = $1 AND day = $2 AND path NOT IN ($3, $4) LIMIT $5",
		key.siteID, key.day, PathNotFound, PathOther, limit)
	if err != nil {
		return nil, fmt.Errorf("list stored analytics paths: %w", err)
	}
	paths := map[string]bool{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, errors.Join(fmt.Errorf("scan a stored analytics path: %w", err), rows.Close())
		}
		paths[path] = true
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read stored analytics paths: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close stored analytics paths: %w", err)
	}
	return paths, nil
}
