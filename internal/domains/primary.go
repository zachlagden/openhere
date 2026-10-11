package domains

import (
	"context"
	"fmt"

	"github.com/zachlagden/openhere/internal/store"
)

const primaryAttempts = 3

func (s *Service) SetPrimary(ctx context.Context, siteID, id string, primary bool) (Domain, error) {
	var result Domain
	var err error
	for range primaryAttempts {
		err = store.WithTx(ctx, s.db, func(q store.Querier) error {
			var txErr error
			result, txErr = changePrimary(ctx, q, siteID, id, primary)
			return txErr
		})
		if !store.IsUniqueViolation(err) {
			break
		}
	}
	if err != nil {
		return Domain{}, err
	}
	s.cache.invalidate()
	return result, nil
}

func lockSite(ctx context.Context, q store.Querier, siteID string) error {
	if _, err := q.ExecContext(ctx, "UPDATE sites SET id = id WHERE id = $1", siteID); err != nil {
		return fmt.Errorf("lock site: %w", err)
	}
	return nil
}

func changePrimary(ctx context.Context, q store.Querier, siteID, id string, primary bool) (Domain, error) {
	if err := lockSite(ctx, q, siteID); err != nil {
		return Domain{}, err
	}
	current, err := getDomain(ctx, q, siteID, id)
	if err != nil {
		return Domain{}, err
	}
	if !primary {
		return clearPrimary(ctx, q, current)
	}
	if !current.Verified() {
		return Domain{}, ErrNotVerified
	}
	if _, err := q.ExecContext(ctx,
		"UPDATE site_domains SET is_primary = FALSE, primary_site_id = NULL WHERE site_id = $1 AND is_primary = TRUE", siteID); err != nil {
		return Domain{}, fmt.Errorf("clear primary domain: %w", err)
	}
	result, err := q.ExecContext(ctx,
		"UPDATE site_domains SET is_primary = TRUE, primary_site_id = $1 WHERE id = $2 AND site_id = $1 AND verified_at IS NOT NULL", siteID, id)
	if err != nil {
		return Domain{}, fmt.Errorf("set primary domain: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Domain{}, fmt.Errorf("set primary domain: %w", err)
	}
	if changed != 1 {
		return Domain{}, ErrNotVerified
	}
	return getDomain(ctx, q, siteID, id)
}

func clearPrimary(ctx context.Context, q store.Querier, current Domain) (Domain, error) {
	if _, err := q.ExecContext(ctx,
		"UPDATE site_domains SET is_primary = FALSE, primary_site_id = NULL WHERE id = $1 AND site_id = $2", current.ID, current.SiteID); err != nil {
		return Domain{}, fmt.Errorf("clear primary domain: %w", err)
	}
	return getDomain(ctx, q, current.SiteID, current.ID)
}
