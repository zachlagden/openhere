package domains

import (
	"context"
	"errors"
	"fmt"

	"github.com/zachlagden/openhere/internal/ids"
	"github.com/zachlagden/openhere/internal/store"
)

func (s *Service) Claim(ctx context.Context, siteID, raw string) (Domain, error) {
	name, err := Normalise(raw)
	if err != nil {
		return Domain{}, err
	}
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return Domain{}, err
	}
	if err := s.checkReserved(name, cfg); err != nil {
		return Domain{}, err
	}
	domain, err := s.newDomain(siteID, name)
	if err != nil {
		return Domain{}, err
	}
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		return insertWithinLimit(ctx, q, domain, cfg.MaxDomainsPerSite)
	})
	if store.IsUniqueViolation(err) || errors.Is(err, ErrTaken) {
		return Domain{}, ErrTaken
	}
	if err != nil {
		return Domain{}, err
	}
	s.cache.invalidate()
	return domain, nil
}

func (s *Service) newDomain(siteID, name string) (Domain, error) {
	id, err := ids.New()
	if err != nil {
		return Domain{}, err
	}
	token, err := newToken()
	if err != nil {
		return Domain{}, fmt.Errorf("generate challenge token: %w", err)
	}
	return Domain{ID: id, SiteID: siteID, Name: name, Token: token, CreatedAt: s.clock()}, nil
}

func insertWithinLimit(ctx context.Context, q store.Querier, d Domain, limit int) error {
	if err := lockSite(ctx, q, d.SiteID); err != nil {
		return err
	}
	var count int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM site_domains WHERE site_id = $1", d.SiteID).Scan(&count); err != nil {
		return fmt.Errorf("count site domains: %w", err)
	}
	if count >= limit {
		return ErrLimitReached
	}
	var held int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM site_domains WHERE domain = $1 AND verified_at IS NOT NULL", d.Name).Scan(&held)
	if err != nil {
		return fmt.Errorf("check verified domain: %w", err)
	}
	if held > 0 {
		return ErrTaken
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO site_domains (id, site_id, domain, challenge_token, last_check_result, missing_checks, is_primary, created_at, claim_started_at)
		VALUES ($1, $2, $3, $4, '', 0, FALSE, $5, $5)`,
		d.ID, d.SiteID, d.Name, d.Token, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert domain: %w", err)
	}
	return nil
}
