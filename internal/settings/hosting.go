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

var ErrWildcardNotVerified = errors.New("subdomain mode needs a successful wildcard check")

func (s *Service) applyHostingRules(ctx context.Context, q store.Querier, changes map[string]string) error {
	status, err := storedValue(ctx, q, KeyWildcardStatus, WildcardUnchecked)
	if err != nil {
		return err
	}
	if newDomain, changing := changes[KeyBaseDomain]; changing {
		current, err := storedValue(ctx, q, KeyBaseDomain, "")
		if err != nil {
			return err
		}
		if s.effectiveDomain(newDomain) != s.effectiveDomain(current) {
			status = WildcardUnchecked
			resetWildcard(changes)
		}
	}
	if changes[KeySubdomainEnabled] == strconv.FormatBool(true) && status != WildcardOK {
		return ErrWildcardNotVerified
	}
	return nil
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
