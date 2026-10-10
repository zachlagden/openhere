package domains

import (
	"database/sql"
	"fmt"
	"time"
)

const columns = "id, site_id, domain, challenge_token, verified_at, last_checked_at, last_check_result, missing_checks, is_primary, created_at"

type scanner interface {
	Scan(dest ...any) error
}

func scanDomain(row scanner) (Domain, error) {
	var d Domain
	var verified, checked sql.NullTime
	err := row.Scan(&d.ID, &d.SiteID, &d.Name, &d.Token, &verified, &checked, &d.LastCheckResult, &d.MissingChecks, &d.IsPrimary, &d.CreatedAt)
	if err != nil {
		return Domain{}, err
	}
	d.VerifiedAt = nullTime(verified)
	d.LastCheckedAt = nullTime(checked)
	return d, nil
}

func nullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	utc := value.Time.UTC()
	return &utc
}

func collect(rows *sql.Rows) ([]Domain, error) {
	var out []Domain
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			return nil, closeAfter(rows, fmt.Errorf("scan domain: %w", err))
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, closeAfter(rows, fmt.Errorf("read domains: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close domain rows: %w", err)
	}
	return out, nil
}

func closeAfter(rows *sql.Rows, err error) error {
	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("%w (and closing rows failed: %w)", err, closeErr)
	}
	return err
}
