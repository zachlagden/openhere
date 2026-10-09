package access

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zachlagden/selfhost/internal/store"
)

func (s *Service) Get(ctx context.Context, siteID string) (Config, error) {
	current, _, err := loadConfig(ctx, s.db, siteID)
	return current, err
}

func loadConfig(ctx context.Context, q store.Querier, siteID string) (Config, sql.NullString, error) {
	var mode string
	var hash sql.NullString
	err := q.QueryRowContext(ctx, "SELECT access_mode, access_password_hash FROM sites WHERE id = $1", siteID).
		Scan(&mode, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, sql.NullString{}, ErrSiteNotFound
	}
	if err != nil {
		return Config{}, sql.NullString{}, wrap("load site access", err)
	}
	emails, err := loadList(ctx, q, "SELECT email FROM site_access_emails WHERE site_id = $1 ORDER BY email", siteID)
	if err != nil {
		return Config{}, sql.NullString{}, err
	}
	domains, err := loadList(ctx, q, "SELECT domain FROM site_access_domains WHERE site_id = $1 ORDER BY domain", siteID)
	if err != nil {
		return Config{}, sql.NullString{}, err
	}
	return Config{Mode: Mode(mode), HasPassword: hash.Valid, Emails: emails, Domains: domains}, hash, nil
}

func loadList(ctx context.Context, q store.Querier, query, siteID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, siteID)
	if err != nil {
		return nil, wrap("load allowlist", err)
	}
	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, errors.Join(wrap("scan allowlist", err), rows.Close())
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(wrap("read allowlist", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, wrap("close allowlist rows", err)
	}
	return values, nil
}
