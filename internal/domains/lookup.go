package domains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func plausibleHost(host string) bool {
	return len(host) >= 4 && len(host) <= maxNameLength && strings.Contains(host, ".")
}

func (s *Service) Lookup(ctx context.Context, host string) (Target, bool, error) {
	if !plausibleHost(host) {
		return Target{}, false, nil
	}
	if entry, ok := s.cache.get(host); ok {
		return entry.target, entry.serves, nil
	}
	generation := s.cache.currentGeneration()
	target, serves, err := s.load(ctx, host)
	if err != nil {
		return Target{}, false, err
	}
	s.cache.put(host, target, serves, generation)
	return target, serves, nil
}

func (s *Service) load(ctx context.Context, host string) (Target, bool, error) {
	target := Target{Domain: host}
	err := s.db.QueryRowContext(ctx,
		`SELECT d.site_id, s.slug FROM site_domains d JOIN sites s ON s.id = d.site_id
		WHERE d.domain = $1 AND d.verified_at IS NOT NULL`, host).Scan(&target.SiteID, &target.Slug)
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, fmt.Errorf("look up custom domain: %w", err)
	}
	return target, true, nil
}
