package domains

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

func (s *Service) Verify(ctx context.Context, siteID, id string) (Domain, error) {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return Domain{}, err
	}
	result := s.check(ctx, current)
	now := s.clock()
	if result == ResultVerified && !current.Verified() {
		err = s.markVerified(ctx, current, now)
	} else {
		err = s.recordResult(ctx, current, result, now)
	}
	if err != nil {
		return Domain{}, err
	}
	s.cache.invalidate()
	return s.Get(ctx, siteID, id)
}

func (s *Service) markVerified(ctx context.Context, d Domain, now time.Time) error {
	err := store.WithTx(ctx, s.db, func(q store.Querier) error {
		if _, err := q.ExecContext(ctx,
			"DELETE FROM site_domains WHERE domain = $1 AND verified_at IS NULL AND id <> $2", d.Name, d.ID); err != nil {
			return fmt.Errorf("displace competing claims: %w", err)
		}
		res, err := q.ExecContext(ctx,
			`UPDATE site_domains SET verified_at = $1, last_checked_at = $1, last_check_result = $2, missing_checks = 0
			WHERE id = $3 AND site_id = $4`, now, ResultVerified, d.ID, d.SiteID)
		return requireOneRow(res, err, "mark domain verified")
	})
	if store.IsUniqueViolation(err) {
		return ErrTaken
	}
	return err
}

func (s *Service) recordResult(ctx context.Context, d Domain, result string, now time.Time) error {
	query := "UPDATE site_domains SET last_checked_at = $1, last_check_result = $2 WHERE id = $3 AND site_id = $4"
	if result == ResultVerified {
		query = "UPDATE site_domains SET last_checked_at = $1, last_check_result = $2, missing_checks = 0 WHERE id = $3 AND site_id = $4"
	}
	res, err := s.db.ExecContext(ctx, query, now, result, d.ID, d.SiteID)
	return requireOneRow(res, err, "record domain check")
}

func requireOneRow(res sql.Result, err error, action string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}
