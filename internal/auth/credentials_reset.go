package auth

import (
	"context"
	"fmt"

	"github.com/zachlagden/selfhost/internal/store"
)

func (s *Service) HashNewPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := HashPassword(password, s.params)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

func (s *Service) ApplyPasswordHash(ctx context.Context, q store.Querier, userID, hash string) error {
	result, err := q.ExecContext(ctx,
		"UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3 AND disabled_at IS NULL", hash, s.clock(), userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if changed != 1 {
		return ErrUserNotFound
	}
	if _, err := q.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("end sessions: %w", err)
	}
	return nil
}
