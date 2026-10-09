package access

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Service) emailAllowed(ctx context.Context, siteID, email string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM site_access_emails WHERE site_id = $1 AND email = $2
		UNION ALL SELECT 1 FROM site_access_domains WHERE site_id = $1 AND domain = $3 LIMIT 1`,
		siteID, email, domainOf(email)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, wrap("check allowlist", err)
	}
	return true, nil
}
