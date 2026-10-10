package domains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) List(ctx context.Context, siteID string) ([]Domain, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+columns+" FROM site_domains WHERE site_id = $1 ORDER BY created_at, id", siteID)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	return collect(rows)
}

func (s *Service) Get(ctx context.Context, siteID, id string) (Domain, error) {
	return getDomain(ctx, s.db, siteID, id)
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getDomain(ctx context.Context, q rowQuerier, siteID, id string) (Domain, error) {
	row := q.QueryRowContext(ctx, "SELECT "+columns+" FROM site_domains WHERE id = $1 AND site_id = $2", id, siteID)
	d, err := scanDomain(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Domain{}, ErrNotFound
	}
	if err != nil {
		return Domain{}, fmt.Errorf("load domain: %w", err)
	}
	return d, nil
}

func (s *Service) Summaries(ctx context.Context, siteIDs []string) (map[string]Summary, error) {
	out := make(map[string]Summary, len(siteIDs))
	if len(siteIDs) == 0 {
		return out, nil
	}
	marks := make([]string, len(siteIDs))
	args := make([]any, len(siteIDs))
	for i, id := range siteIDs {
		marks[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT site_id, domain, verified_at IS NOT NULL, is_primary FROM site_domains WHERE site_id IN ("+strings.Join(marks, ", ")+")", args...)
	if err != nil {
		return nil, fmt.Errorf("summarise domains: %w", err)
	}
	for rows.Next() {
		var siteID, name string
		var verified, primary bool
		if err := rows.Scan(&siteID, &name, &verified, &primary); err != nil {
			return nil, closeAfter(rows, fmt.Errorf("scan domain summary: %w", err))
		}
		summary := out[siteID]
		summary.Total++
		if verified {
			summary.Verified++
		}
		if primary {
			summary.Primary = name
		}
		out[siteID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, closeAfter(rows, fmt.Errorf("read domain summaries: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close domain summary rows: %w", err)
	}
	return out, nil
}
