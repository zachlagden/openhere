package passwordreset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/mail"
	"github.com/zachlagden/openhere/internal/store"
)

func (s *Service) Request(ctx context.Context, email string) error {
	if !s.mailer.Configured() {
		return nil
	}
	user, err := s.auth.UserByEmail(ctx, email)
	if errors.Is(err, auth.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.Disabled() {
		return nil
	}
	token, _, err := s.issue(ctx, user.ID)
	if err != nil {
		return err
	}
	s.spawn(func() { s.deliver(user.Email, token) })
	return nil
}

func (s *Service) AdminLink(ctx context.Context, userID string) (string, time.Time, error) {
	user, err := s.auth.UserByID(ctx, userID)
	if err != nil {
		return "", time.Time{}, err
	}
	if user.Disabled() {
		return "", time.Time{}, auth.ErrUserDisabled
	}
	token, expires, err := s.issue(ctx, user.ID)
	if err != nil {
		return "", time.Time{}, err
	}
	return s.linkFor(token), expires, nil
}

func (s *Service) issue(ctx context.Context, userID string) (string, time.Time, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.clock()
	expires := now.Add(Lifetime)
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		if _, err := q.ExecContext(ctx, "DELETE FROM password_resets WHERE user_id = $1 AND used_at IS NULL", userID); err != nil {
			return fmt.Errorf("discard earlier reset links: %w", err)
		}
		_, err := q.ExecContext(ctx,
			"INSERT INTO password_resets (token_hash, user_id, created_at, expires_at) VALUES ($1, $2, $3, $4)",
			auth.HashToken(token), userID, now, expires)
		if err != nil {
			return fmt.Errorf("store reset link: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Service) deliver(address, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	host := s.baseURL
	if parsed, err := url.Parse(s.baseURL); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	msg := mail.Message{
		To:      address,
		Subject: "Reset your password for " + host,
		Text: fmt.Sprintf("Someone asked to reset the password for your account on %s. To choose a new password, open this link:\n%s\n\nThe link works once and expires in %d minutes. If you did not ask for it, ignore this message and your password stays the same.\n",
			host, s.linkFor(token), int(Lifetime.Minutes())),
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.logger.Warn("send password reset email", "reason", mail.Reason(err))
	}
}

func (s *Service) Confirm(ctx context.Context, token, password string, revokeAPIKeys bool) error {
	userID, err := s.lookup(ctx, token)
	if err != nil {
		return err
	}
	hash, err := s.auth.HashNewPassword(ctx, password)
	if err != nil {
		return err
	}
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		return s.apply(ctx, q, token, userID, hash, revokeAPIKeys)
	})
	if errors.Is(err, auth.ErrUserNotFound) {
		return ErrInvalidToken
	}
	return err
}

func (s *Service) lookup(ctx context.Context, token string) (string, error) {
	if len(token) != auth.TokenLength {
		return "", ErrInvalidToken
	}
	var userID string
	var used sql.NullTime
	var expires time.Time
	err := s.db.QueryRowContext(ctx, "SELECT user_id, used_at, expires_at FROM password_resets WHERE token_hash = $1", auth.HashToken(token)).
		Scan(&userID, &used, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("load reset link: %w", err)
	}
	if used.Valid || !expires.After(s.clock()) {
		return "", ErrInvalidToken
	}
	return userID, nil
}

func (s *Service) apply(ctx context.Context, q store.Querier, token, userID, hash string, revokeAPIKeys bool) error {
	claimed, err := q.ExecContext(ctx,
		"UPDATE password_resets SET used_at = $1 WHERE token_hash = $2 AND used_at IS NULL AND expires_at > $1", s.clock(), auth.HashToken(token))
	if err != nil {
		return fmt.Errorf("claim reset link: %w", err)
	}
	changed, err := claimed.RowsAffected()
	if err != nil {
		return fmt.Errorf("claim reset link: %w", err)
	}
	if changed != 1 {
		return ErrInvalidToken
	}
	if err := s.auth.ApplyPasswordHash(ctx, q, userID, hash); err != nil {
		return err
	}
	if revokeAPIKeys {
		if err := s.auth.RevokeAllAPIKeys(ctx, q, userID); err != nil {
			return err
		}
	}
	if _, err := q.ExecContext(ctx, "DELETE FROM password_resets WHERE user_id = $1 AND used_at IS NULL", userID); err != nil {
		return fmt.Errorf("discard other reset links: %w", err)
	}
	return nil
}

func (s *Service) Purge(ctx context.Context) error {
	cutoff := s.clock().Add(-24 * time.Hour)
	if _, err := s.db.ExecContext(ctx, "DELETE FROM password_resets WHERE expires_at <= $1", cutoff); err != nil {
		return fmt.Errorf("purge reset links: %w", err)
	}
	return nil
}
