package domains

import (
	"context"
	"fmt"
)

func (s *Service) VerifiedNames(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT domain FROM site_domains WHERE verified_at IS NOT NULL ORDER BY domain LIMIT $1", limit)
	if err != nil {
		return nil, fmt.Errorf("list verified custom domains: %w", err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, closeAfter(rows, fmt.Errorf("scan verified custom domain: %w", err))
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, closeAfter(rows, fmt.Errorf("read verified custom domains: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close verified custom domain rows: %w", err)
	}
	return names, nil
}
