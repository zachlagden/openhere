package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	SessionTTL           = 30 * 24 * time.Hour
	sessionTouchInterval = time.Hour
)

type Session struct {
	ID         string
	UserID     string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

type SessionInfo struct {
	Session  Session
	User     User
	Extended bool
}

func (s *Service) CreateSession(ctx context.Context, userID string) (string, Session, error) {
	token, err := NewToken()
	if err != nil {
		return "", Session{}, err
	}
	now := s.clock()
	session := Session{ID: HashToken(token), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(SessionTTL), LastSeenAt: now}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, user_id, created_at, expires_at, last_seen_at) VALUES ($1, $2, $3, $4, $5)",
		session.ID, session.UserID, session.CreatedAt, session.ExpiresAt, session.LastSeenAt)
	if err != nil {
		return "", Session{}, fmt.Errorf("insert session: %w", err)
	}
	return token, session, nil
}

func (s *Service) ValidateSession(ctx context.Context, token string) (SessionInfo, error) {
	if len(token) != encodedToken {
		return SessionInfo{}, ErrSessionInvalid
	}
	id := HashToken(token)
	var info SessionInfo
	var storedID string
	row := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
		u.id, u.email, u.display_name, u.is_root, u.created_at, u.updated_at
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.id = $1 AND u.disabled_at IS NULL`, id)
	err := row.Scan(&storedID, &info.Session.UserID, &info.Session.CreatedAt, &info.Session.ExpiresAt, &info.Session.LastSeenAt,
		&info.User.ID, &info.User.Email, &info.User.DisplayName, &info.User.IsRoot, &info.User.CreatedAt, &info.User.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionInfo{}, ErrSessionInvalid
	}
	if err != nil {
		return SessionInfo{}, fmt.Errorf("load session: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(storedID), []byte(id)) != 1 {
		return SessionInfo{}, ErrSessionInvalid
	}
	info.Session.ID = storedID

	now := s.clock()
	if !info.Session.ExpiresAt.After(now) {
		if err := s.deleteSessionByID(ctx, id); err != nil {
			return SessionInfo{}, err
		}
		return SessionInfo{}, ErrSessionInvalid
	}
	if now.Sub(info.Session.LastSeenAt) >= sessionTouchInterval {
		return s.extendSession(ctx, info, now)
	}
	return info, nil
}

func (s *Service) extendSession(ctx context.Context, info SessionInfo, now time.Time) (SessionInfo, error) {
	expires := now.Add(SessionTTL)
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at = $1, expires_at = $2 WHERE id = $3", now, expires, info.Session.ID)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("extend session: %w", err)
	}
	info.Session.LastSeenAt = now
	info.Session.ExpiresAt = expires
	info.Extended = true
	return info, nil
}

func (s *Service) DeleteSession(ctx context.Context, token string) error {
	if len(token) != encodedToken {
		return nil
	}
	return s.deleteSessionByID(ctx, HashToken(token))
}

func (s *Service) deleteSessionByID(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
