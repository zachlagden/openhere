package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

var (
	ErrWildcardNotVerified = errors.New("subdomain mode needs a successful wildcard check")
	ErrNoHostingMode       = errors.New("at least one hosting mode must stay on")
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
		status = WildcardUnchecked
		resetWildcard(changes)
	}
	if changes[KeySubdomainEnabled] == strconv.FormatBool(true) && status != WildcardOK {
		return nil, ErrWildcardNotVerified
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

func (s *Service) RecordWildcard(ctx context.Context, verified bool, detail string) (Settings, error) {
	status := WildcardFailed
	if verified {
		status = WildcardOK
	}
	changes := map[string]string{
		KeyWildcardStatus:  status,
		KeyWildcardChecked: s.now().UTC().Truncate(time.Second).Format(time.RFC3339),
		KeyWildcardDetail:  detail,
	}
	if !verified {
		changes[KeySubdomainEnabled] = strconv.FormatBool(false)
	}
	err := store.WithTx(ctx, s.db, func(q store.Querier) error { return s.write(ctx, q, changes) })
	s.invalidate()
	if err != nil {
		return Settings{}, err
	}
	return s.Get(ctx)
}
