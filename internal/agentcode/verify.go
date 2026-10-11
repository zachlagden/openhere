package agentcode

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/otp"
)

func (s *Service) Verify(ctx context.Context, email, code, label string) (string, auth.APIKey, error) {
	if strings.TrimSpace(label) == "" {
		label = DefaultLabel
	}
	cleanLabel, err := auth.NormalizeLabel(label)
	if err != nil {
		return "", auth.APIKey{}, err
	}
	normalised, err := auth.NormalizeEmail(email)
	if err != nil || !otp.ValidShape(code, CodeDigits) {
		return "", auth.APIKey{}, ErrCodeInvalid
	}
	matched, err := s.consume(ctx, normalised, code)
	if err != nil {
		return "", auth.APIKey{}, err
	}
	if !matched {
		return "", auth.APIKey{}, ErrCodeInvalid
	}
	user, err := s.auth.UserByEmail(ctx, normalised)
	if errors.Is(err, auth.ErrUserNotFound) {
		return "", auth.APIKey{}, ErrCodeInvalid
	}
	if err != nil {
		return "", auth.APIKey{}, err
	}
	if user.Disabled() {
		return "", auth.APIKey{}, ErrCodeInvalid
	}
	return s.auth.CreateAPIKey(ctx, user.ID, cleanLabel)
}

func (s *Service) consume(ctx context.Context, email, code string) (bool, error) {
	held, err := s.reserveAttempt(ctx, email)
	if err != nil || !held {
		return false, err
	}
	var stored string
	var attempts int
	err = s.db.QueryRowContext(ctx, "SELECT code_hash, attempts FROM agent_codes WHERE email = $1", email).Scan(&stored, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load agent code: %w", err)
	}
	if !hmac.Equal([]byte(s.hash(email, code)), []byte(stored)) {
		return false, s.dropIfSpent(ctx, email, attempts)
	}
	return s.spend(ctx, email, stored)
}

func (s *Service) reserveAttempt(ctx context.Context, email string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"UPDATE agent_codes SET attempts = attempts + 1 WHERE email = $1 AND attempts < $2 AND expires_at > $3",
		email, CodeAttempts, s.clock())
	if err != nil {
		return false, fmt.Errorf("count agent code attempt: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count agent code attempt: %w", err)
	}
	return rows == 1, nil
}

func (s *Service) dropIfSpent(ctx context.Context, email string, attempts int) error {
	if attempts < CodeAttempts {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM agent_codes WHERE email = $1", email); err != nil {
		return fmt.Errorf("drop spent agent code: %w", err)
	}
	return nil
}

func (s *Service) spend(ctx context.Context, email, hash string) (bool, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM agent_codes WHERE email = $1 AND code_hash = $2", email, hash)
	if err != nil {
		return false, fmt.Errorf("spend agent code: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("spend agent code: %w", err)
	}
	return rows == 1, nil
}
