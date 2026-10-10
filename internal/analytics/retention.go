package analytics

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) Purge(ctx context.Context) error {
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	today := s.clock()
	rawCutoff := today.AddDate(0, 0, -cfg.Analytics.RawDays).Format(DayLayout)
	if err := s.purgeRaw(ctx, rawCutoff); err != nil {
		return err
	}
	if cfg.Analytics.RetentionDays == 0 {
		return nil
	}
	return s.purgeBefore(ctx, today.AddDate(0, 0, -cfg.Analytics.RetentionDays))
}

func (s *Service) purgeRaw(ctx context.Context, cutoff string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM analytics_events WHERE day < $1 AND EXISTS (
			SELECT 1 FROM analytics_daily d WHERE d.site_id = analytics_events.site_id AND d.day = analytics_events.day AND d.dimension = $2)`,
		cutoff, dimEvents)
	if err != nil {
		return fmt.Errorf("purge raw analytics: %w", err)
	}
	return nil
}

func (s *Service) purgeBefore(ctx context.Context, cutoff time.Time) error {
	day := cutoff.Format(DayLayout)
	if _, err := s.db.ExecContext(ctx, "DELETE FROM analytics_events WHERE day < $1", day); err != nil {
		return fmt.Errorf("purge analytics events: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM analytics_daily WHERE day < $1", day); err != nil {
		return fmt.Errorf("purge analytics rollups: %w", err)
	}
	return nil
}

func (s *Service) Maintain(ctx context.Context) error {
	if err := s.Rollup(ctx); err != nil {
		return err
	}
	return s.Purge(ctx)
}
