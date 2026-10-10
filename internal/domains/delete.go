package domains

import (
	"context"
	"fmt"
)

func (s *Service) Delete(ctx context.Context, siteID, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM site_domains WHERE id = $1 AND site_id = $2", id, siteID)
	if err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}
	s.cache.invalidate()
	if removed == 0 {
		return ErrNotFound
	}
	return nil
}
