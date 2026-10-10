package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/zachlagden/selfhost/internal/store"
)

var (
	ErrWildcardNotVerified = errors.New("subdomain mode needs a successful wildcard check")
	ErrNoHostingMode       = errors.New("at least one hosting mode must stay on")
	ErrBaseDomainInUse     = errors.New("a custom domain already uses that base domain or a name under it")
)

const NoticePathModeEnabled = "path_mode_enabled"

type Notice struct {
	Code    string
	Message string
}

func (s *Service) applyHostingRules(ctx context.Context, q store.Querier, changes map[string]string) ([]Notice, error) {
	status, err := storedValue(ctx, q, KeyWildcardStatus, WildcardUnchecked)
	if err != nil {
		return nil, err
	}
	if err := requireAMode(ctx, q, changes); err != nil {
		return nil, err
	}
	moved, err := s.baseDomainMoves(ctx, q, changes)
	if err != nil {
		return nil, err
	}
	if moved {
		if err := requireNoDomainUnder(ctx, q, s.effectiveDomain(changes[KeyBaseDomain])); err != nil {
			return nil, err
		}
		status = WildcardUnchecked
		resetWildcard(changes)
	}
	if changes[KeySubdomainEnabled] == strconv.FormatBool(true) && status != WildcardOK {
		return nil, ErrWildcardNotVerified
	}
	if _, switching := changes[KeySubdomainEnabled]; switching {
		changes[KeyDisabledReason] = ""
	}
	if !moved {
		return nil, nil
	}
	return keepAModeOn(ctx, q, changes)
}

func (s *Service) baseDomainMoves(ctx context.Context, q store.Querier, changes map[string]string) (bool, error) {
	newDomain, changing := changes[KeyBaseDomain]
	if !changing {
		return false, nil
	}
	current, err := storedValue(ctx, q, KeyBaseDomain, "")
	if err != nil {
		return false, err
	}
	return s.effectiveDomain(newDomain) != s.effectiveDomain(current), nil
}

func requireAMode(ctx context.Context, q store.Querier, changes map[string]string) error {
	subdomain, path, err := effectiveModes(ctx, q, changes)
	if err != nil {
		return err
	}
	if !subdomain && !path {
		return ErrNoHostingMode
	}
	return nil
}

func keepAModeOn(ctx context.Context, q store.Querier, changes map[string]string) ([]Notice, error) {
	subdomain, path, err := effectiveModes(ctx, q, changes)
	if err != nil {
		return nil, err
	}
	if subdomain || path {
		return nil, nil
	}
	changes[KeyPathEnabled] = strconv.FormatBool(true)
	return []Notice{{
		Code:    NoticePathModeEnabled,
		Message: "Path mode was switched on because changing the base domain switched subdomain mode off and one mode must stay on.",
	}}, nil
}

func effectiveModes(ctx context.Context, q store.Querier, changes map[string]string) (subdomain, path bool, err error) {
	subdomain, err = effectiveFlag(ctx, q, changes, KeySubdomainEnabled, false)
	if err != nil {
		return false, false, err
	}
	path, err = effectiveFlag(ctx, q, changes, KeyPathEnabled, true)
	if err != nil {
		return false, false, err
	}
	return subdomain, path, nil
}

func effectiveFlag(ctx context.Context, q store.Querier, changes map[string]string, key string, fallback bool) (bool, error) {
	raw, ok := changes[key]
	if !ok {
		var err error
		raw, err = storedValue(ctx, q, key, strconv.FormatBool(fallback))
		if err != nil {
			return false, err
		}
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("setting %s holds %q, which is not true or false", key, raw)
	}
	return value, nil
}

func (s *Service) effectiveDomain(stored string) string {
	if stored == "" {
		return s.defaultDomain
	}
	return stored
}

func resetWildcard(changes map[string]string) {
	changes[KeyWildcardStatus] = WildcardUnchecked
	changes[KeyWildcardChecked] = ""
	changes[KeyWildcardDetail] = ""
	changes[KeyWildcardFailures] = "0"
	changes[KeyDisabledReason] = ""
	if changes[KeySubdomainEnabled] != strconv.FormatBool(true) {
		changes[KeySubdomainEnabled] = strconv.FormatBool(false)
	}
}

func storedValue(ctx context.Context, q store.Querier, key, fallback string) (string, error) {
	var value string
	err := q.QueryRowContext(ctx, "SELECT value FROM instance_settings WHERE key = $1", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return "", fmt.Errorf("read setting %s: %w", key, err)
	}
	return value, nil
}

func requireNoDomainUnder(ctx context.Context, q store.Querier, baseDomain string) error {
	if baseDomain == "" {
		return nil
	}
	var one int
	err := q.QueryRowContext(ctx, "SELECT 1 FROM site_domains WHERE domain = $1 OR domain LIKE $2 LIMIT 1", baseDomain, "%."+baseDomain).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check custom domains against base domain: %w", err)
	}
	return ErrBaseDomainInUse
}
