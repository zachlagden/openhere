package publish

import (
	"context"
	"errors"
	"fmt"
)

const (
	MaxPendingPerSite = 3
	MaxPendingPerUser = 10
)

type PendingLimit string

const (
	LimitSite  PendingLimit = "site"
	LimitUser  PendingLimit = "user"
	LimitBytes PendingLimit = "bytes"
)

type PendingLimitError struct {
	Limit PendingLimit
}

func (e *PendingLimitError) Error() string {
	return fmt.Sprintf("too many pending versions (%s limit)", e.Limit)
}

func (s *Service) reservePending(ctx context.Context, siteID, userID string, newBytes, maxBytes int64) error {
	err := s.checkPending(ctx, siteID, userID, newBytes, maxBytes)
	var limit *PendingLimitError
	if !errors.As(err, &limit) {
		return err
	}
	if _, expireErr := s.expirePending(ctx); expireErr != nil {
		s.logger.Warn("expire pending versions before refusing a manifest", "error", expireErr)
	}
	return s.checkPending(ctx, siteID, userID, newBytes, maxBytes)
}

func (s *Service) checkPending(ctx context.Context, siteID, userID string, newBytes, maxBytes int64) error {
	bySite, err := s.countPending(ctx, "site_id = $2", siteID)
	if err != nil {
		return err
	}
	if bySite >= MaxPendingPerSite {
		return &PendingLimitError{Limit: LimitSite}
	}
	if userID != "" {
		byUser, err := s.countPending(ctx, "created_by = $2", userID)
		if err != nil {
			return err
		}
		if byUser >= MaxPendingPerUser {
			return &PendingLimitError{Limit: LimitUser}
		}
	}
	if maxBytes <= 0 {
		return nil
	}
	var declared int64
	err = s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(total_bytes), 0) FROM versions WHERE status = $1", StatusPending).Scan(&declared)
	if err != nil {
		return fmt.Errorf("sum pending bytes: %w", err)
	}
	if declared+newBytes > maxBytes {
		return &PendingLimitError{Limit: LimitBytes}
	}
	return nil
}

func (s *Service) countPending(ctx context.Context, condition string, arg any) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM versions WHERE status = $1 AND "+condition, StatusPending, arg).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending versions: %w", err)
	}
	return count, nil
}
