package auth

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < $1", s.clock())
	if err != nil {
		return 0, fmt.Errorf("purge expired sessions: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge expired sessions: %w", err)
	}
	return removed, nil
}

type MaintenanceTask struct {
	Name string
	Run  func(ctx context.Context) error
}

func (s *Service) RunMaintenance(ctx context.Context, interval time.Duration, tasks ...MaintenanceTask) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.purgeSessions(ctx)
			for _, task := range tasks {
				if err := task.Run(ctx); err != nil {
					s.logger.Error("maintenance task failed", "task", task.Name, "error", err)
				}
			}
		}
	}
}

func (s *Service) purgeSessions(ctx context.Context) {
	removed, err := s.PurgeExpiredSessions(ctx)
	if err != nil {
		s.logger.Error("purge expired sessions", "error", err)
		return
	}
	if removed > 0 {
		s.logger.Info("purged expired sessions", "count", removed)
	}
}
