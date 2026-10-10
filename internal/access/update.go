package access

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/validate"
)

type plan struct {
	mode        *Mode
	setPassword bool
	hash        sql.NullString
	emails      *[]string
	domains     *[]string
}

func (s *Service) Update(ctx context.Context, siteID string, in ConfigUpdate) (Config, error) {
	prepared, err := s.prepare(ctx, in)
	if err != nil {
		return Config{}, err
	}
	var result Config
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		var applyErr error
		result, applyErr = applyPlan(ctx, q, siteID, prepared)
		return applyErr
	})
	if err != nil {
		return Config{}, err
	}
	return result, nil
}

func (s *Service) prepare(ctx context.Context, in ConfigUpdate) (plan, error) {
	var p plan
	if in.Mode != nil {
		mode, ok := ParseMode(*in.Mode)
		if !ok {
			return plan{}, validate.Invalid("Mode must be public, password or restricted.")
		}
		p.mode = &mode
	}
	if err := s.preparePassword(ctx, &p, in.Password); err != nil {
		return plan{}, err
	}
	if in.Emails != nil {
		emails, err := normalizeEmails(*in.Emails)
		if err != nil {
			return plan{}, err
		}
		p.emails = &emails
	}
	if in.Domains != nil {
		domains, err := normalizeDomains(*in.Domains)
		if err != nil {
			return plan{}, err
		}
		p.domains = &domains
	}
	return p, nil
}

func (s *Service) preparePassword(ctx context.Context, p *plan, change PasswordChange) error {
	if !change.Set {
		return nil
	}
	p.setPassword = true
	if change.Value == nil {
		return nil
	}
	length := utf8.RuneCountInString(*change.Value)
	if length < MinPasswordLength || length > MaxPasswordLength {
		return validate.Invalid(fmt.Sprintf("Password must be %d to %d characters long.", MinPasswordLength, MaxPasswordLength))
	}
	hash, err := auth.HashPassword(ctx, *change.Value, s.params)
	if err != nil {
		return wrap("hash site password", err)
	}
	p.hash = sql.NullString{String: hash, Valid: true}
	return nil
}

func applyPlan(ctx context.Context, q store.Querier, siteID string, p plan) (Config, error) {
	current, currentHash, err := loadConfig(ctx, q, siteID)
	if err != nil {
		return Config{}, err
	}
	next := current
	hash := currentHash
	changed := false
	if p.mode != nil && *p.mode != current.Mode {
		next.Mode = *p.mode
		changed = true
	}
	if p.setPassword && p.hash != currentHash {
		hash = p.hash
		changed = true
	}
	if next.Mode == ModePassword && !hash.Valid {
		return Config{}, validate.Invalid("Set a password before using password mode.")
	}
	listsChanged, err := replaceLists(ctx, q, siteID, current, p)
	if err != nil {
		return Config{}, err
	}
	if !changed && !listsChanged {
		return current, nil
	}
	if err := persist(ctx, q, siteID, next.Mode, hash); err != nil {
		return Config{}, err
	}
	reloaded, _, err := loadConfig(ctx, q, siteID)
	return reloaded, err
}

func replaceLists(ctx context.Context, q store.Querier, siteID string, current Config, p plan) (bool, error) {
	changed := false
	if p.emails != nil && !slices.Equal(*p.emails, current.Emails) {
		if err := replaceList(ctx, q, "site_access_emails", "email", siteID, *p.emails); err != nil {
			return false, err
		}
		changed = true
	}
	if p.domains != nil && !slices.Equal(*p.domains, current.Domains) {
		if err := replaceList(ctx, q, "site_access_domains", "domain", siteID, *p.domains); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func replaceList(ctx context.Context, q store.Querier, table, column, siteID string, values []string) error {
	if _, err := q.ExecContext(ctx, "DELETE FROM "+table+" WHERE site_id = $1", siteID); err != nil {
		return wrap("clear allowlist", err)
	}
	for _, value := range values {
		if _, err := q.ExecContext(ctx, "INSERT INTO "+table+" (site_id, "+column+") VALUES ($1, $2)", siteID, value); err != nil {
			return wrap("store allowlist entry", err)
		}
	}
	return nil
}

func persist(ctx context.Context, q store.Querier, siteID string, mode Mode, hash sql.NullString) error {
	_, err := q.ExecContext(ctx,
		"UPDATE sites SET access_mode = $1, access_password_hash = $2, access_epoch = access_epoch + 1 WHERE id = $3",
		string(mode), hash, siteID)
	if err != nil {
		return wrap("update site access", err)
	}
	if _, err := q.ExecContext(ctx, "DELETE FROM viewer_codes WHERE site_id = $1", siteID); err != nil {
		return wrap("discard codes", err)
	}
	if _, err := q.ExecContext(ctx, "DELETE FROM viewer_handoffs WHERE site_id = $1", siteID); err != nil {
		return wrap("discard handoffs", err)
	}
	return nil
}
