package settings

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

const AutoDisableAfterFailures = 3

func (s *Service) RecordWildcard(ctx context.Context, verified bool, detail string) (Settings, error) {
	return s.recordWildcard(ctx, verified, detail, false)
}

func (s *Service) RecordScheduledWildcard(ctx context.Context, verified bool, detail string) (Settings, error) {
	return s.recordWildcard(ctx, verified, detail, true)
}

func (s *Service) recordWildcard(ctx context.Context, verified bool, detail string, scheduled bool) (Settings, error) {
	status := WildcardFailed
	if verified {
		status = WildcardOK
	}
	changes := map[string]string{
		KeyWildcardStatus:  status,
		KeyWildcardChecked: s.now().UTC().Truncate(time.Second).Format(time.RFC3339),
		KeyWildcardDetail:  detail,
	}
	if verified {
		changes[KeyWildcardFailures] = "0"
		changes[KeyDisabledReason] = ""
	}
	err := store.WithTx(ctx, s.db, func(q store.Querier) error {
		if !verified && scheduled {
			if err := countFailure(ctx, q, changes); err != nil {
				return err
			}
		}
		return s.write(ctx, q, changes)
	})
	s.invalidate()
	if err != nil {
		return Settings{}, err
	}
	return s.Get(ctx)
}

func countFailure(ctx context.Context, q store.Querier, changes map[string]string) error {
	subdomain, path, err := effectiveModes(ctx, q, changes)
	if err != nil {
		return err
	}
	if !subdomain {
		return nil
	}
	failures, err := storedInt(ctx, q, KeyWildcardFailures)
	if err != nil {
		return err
	}
	failures++
	changes[KeyWildcardFailures] = strconv.FormatInt(failures, 10)
	if failures >= AutoDisableAfterFailures && path {
		changes[KeySubdomainEnabled] = strconv.FormatBool(false)
		changes[KeyDisabledReason] = DisabledByWildcard
	}
	return nil
}

func storedInt(ctx context.Context, q store.Querier, key string) (int64, error) {
	raw, err := storedValue(ctx, q, key, "0")
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("setting %s holds %q, which is not a whole number", key, raw)
	}
	return value, nil
}
