package domains

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

const recheckBatch = 200

func (s *Service) Maintain(ctx context.Context) error {
	var problems []error
	released, err := s.ReleaseStale(ctx)
	if err != nil {
		problems = append(problems, err)
	}
	if released > 0 {
		s.logger.Info("released stale domain claims", "count", released)
	}
	if err := s.Recheck(ctx); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

func (s *Service) ReleaseStale(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM site_domains WHERE verified_at IS NULL AND claim_started_at < $1", s.clock().Add(-StaleClaimAge))
	if err != nil {
		return 0, fmt.Errorf("release stale domain claims: %w", err)
	}
	released, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("release stale domain claims: %w", err)
	}
	if released > 0 {
		s.cache.invalidate()
	}
	return released, nil
}

func (s *Service) Recheck(ctx context.Context) error {
	due, err := s.dueForRecheck(ctx)
	if err != nil {
		return err
	}
	var problems []error
	for _, d := range due {
		if ctx.Err() != nil {
			problems = append(problems, ctx.Err())
			break
		}
		if err := s.recheckOne(ctx, d); err != nil {
			problems = append(problems, err)
		}
	}
	if len(due) > 0 {
		s.cache.invalidate()
	}
	return errors.Join(problems...)
}

func (s *Service) dueForRecheck(ctx context.Context) ([]Domain, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+columns+` FROM site_domains WHERE verified_at IS NOT NULL AND (last_checked_at IS NULL OR last_checked_at <= $1)
		ORDER BY COALESCE(last_checked_at, verified_at) LIMIT $2`, s.clock().Add(-RecheckInterval), recheckBatch)
	if err != nil {
		return nil, fmt.Errorf("list domains due for recheck: %w", err)
	}
	return collect(rows)
}

func (s *Service) recheckOne(ctx context.Context, d Domain) error {
	result := s.check(ctx, d)
	now := s.clock()
	err := store.WithTx(ctx, s.db, func(q store.Querier) error {
		return applyRecheck(ctx, q, d, result, now)
	})
	if err != nil {
		return fmt.Errorf("recheck domain: %w", err)
	}
	if result != ResultVerified && !countsAsMissing(result) {
		s.logger.Warn("domain recheck could not reach DNS", "domain", d.Name)
	}
	return nil
}

func applyRecheck(ctx context.Context, q store.Querier, d Domain, result string, now time.Time) error {
	switch {
	case result == ResultVerified:
		_, err := q.ExecContext(ctx,
			`UPDATE site_domains SET last_checked_at = $1, last_check_result = $2, missing_checks = 0
			WHERE id = $3 AND verified_at IS NOT NULL`, now, result, d.ID)
		return err
	case !countsAsMissing(result):
		_, err := q.ExecContext(ctx,
			"UPDATE site_domains SET last_checked_at = $1, last_check_result = $2 WHERE id = $3", now, result, d.ID)
		return err
	default:
		return countMiss(ctx, q, d.ID, result, now)
	}
}

func countMiss(ctx context.Context, q store.Querier, id, result string, now time.Time) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE site_domains SET missing_checks = missing_checks + 1, last_checked_at = $1, last_check_result = $2
		WHERE id = $3 AND verified_at IS NOT NULL`, now, result, id); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx,
		`UPDATE site_domains SET verified_at = NULL, is_primary = FALSE, primary_site_id = NULL, missing_checks = 0, claim_started_at = $1
		WHERE id = $2 AND verified_at IS NOT NULL AND missing_checks >= $3`, now, id, MissingChecksToDrop)
	return err
}
