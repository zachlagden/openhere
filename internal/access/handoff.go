package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const handoffTokenBytes = 32

func (s *Service) IssueHandoff(ctx context.Context, siteID, userID string) (string, error) {
	raw := make([]byte, handoffTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", wrap("generate handoff token", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO viewer_handoffs (token_hash, site_id, user_id, expires_at) VALUES ($1, $2, $3, $4)",
		hashHandoff(token), siteID, userID, s.clock().Add(HandoffLifetime))
	if err != nil {
		return "", wrap("store handoff token", err)
	}
	return token, nil
}

func (s *Service) ConsumeHandoff(ctx context.Context, siteID, token string) (State, string, bool, error) {
	state, err := s.State(ctx, siteID)
	if err != nil {
		return State{}, "", false, err
	}
	hash := hashHandoff(token)
	var userID string
	var expires time.Time
	err = s.db.QueryRowContext(ctx, "SELECT user_id, expires_at FROM viewer_handoffs WHERE token_hash = $1 AND site_id = $2", hash, siteID).
		Scan(&userID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return state, "", false, nil
	}
	if err != nil {
		return State{}, "", false, wrap("load handoff token", err)
	}
	spent, err := s.spendHandoff(ctx, hash)
	if err != nil || !spent {
		return state, "", false, err
	}
	if !expires.After(s.clock()) || state.Mode != ModeRestricted {
		return state, "", false, nil
	}
	allowed, err := s.UserCanRead(ctx, userID, siteID)
	return state, userID, allowed, err
}

func (s *Service) spendHandoff(ctx context.Context, hash string) (bool, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM viewer_handoffs WHERE token_hash = $1", hash)
	if err != nil {
		return false, wrap("spend handoff token", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, wrap("spend handoff token", err)
	}
	return rows == 1, nil
}

func hashHandoff(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
