package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

const (
	MaxActiveAPIKeys    = 50
	keyDisplayChars     = 6
	keyLastUsedInterval = time.Minute
)

type APIKey struct {
	ID         string
	UserID     string
	Label      string
	Prefix     string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

const apiKeyColumns = "id, user_id, label, prefix, created_at, last_used_at, revoked_at"

func scanAPIKey(row interface{ Scan(dest ...any) error }) (APIKey, error) {
	var k APIKey
	var lastUsed, revoked sql.NullTime
	if err := row.Scan(&k.ID, &k.UserID, &k.Label, &k.Prefix, &k.CreatedAt, &lastUsed, &revoked); err != nil {
		return APIKey{}, err
	}
	if lastUsed.Valid {
		k.LastUsedAt = &lastUsed.Time
	}
	if revoked.Valid {
		k.RevokedAt = &revoked.Time
	}
	return k, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, userID, label string) (string, APIKey, error) {
	normalized, err := NormalizeLabel(label)
	if err != nil {
		return "", APIKey{}, err
	}
	secret, err := newToken()
	if err != nil {
		return "", APIKey{}, err
	}
	id, err := newID()
	if err != nil {
		return "", APIKey{}, err
	}
	key := s.keyPrefix + secret
	record := APIKey{ID: id, UserID: userID, Label: normalized, Prefix: key[:len(s.keyPrefix)+keyDisplayChars], CreatedAt: s.clock()}

	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		return s.insertAPIKey(ctx, q, record, hashToken(key))
	})
	if err != nil {
		return "", APIKey{}, err
	}
	return key, record, nil
}

func (s *Service) insertAPIKey(ctx context.Context, q store.Querier, record APIKey, keyHash string) error {
	locked, err := q.ExecContext(ctx, "UPDATE users SET id = id WHERE id = $1", record.UserID)
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	if affected, err := locked.RowsAffected(); err != nil || affected == 0 {
		if err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		return ErrUserNotFound
	}
	var active int
	err = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL", record.UserID).Scan(&active)
	if err != nil {
		return fmt.Errorf("count api keys: %w", err)
	}
	if active >= MaxActiveAPIKeys {
		return ErrAPIKeyLimit
	}
	_, err = q.ExecContext(ctx,
		"INSERT INTO api_keys (id, user_id, label, prefix, key_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		record.ID, record.UserID, record.Label, record.Prefix, keyHash, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

func (s *Service) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+apiKeyColumns+" FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC, id", userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	keys := []APIKey{}
	for rows.Next() {
		key, scanErr := scanAPIKey(rows)
		if scanErr != nil {
			return nil, errors.Join(fmt.Errorf("scan api key: %w", scanErr), rows.Close())
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read api keys: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close api key rows: %w", err)
	}
	return keys, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE api_keys SET revoked_at = $1 WHERE id = $2 AND user_id = $3 AND revoked_at IS NULL", s.clock(), keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if affected > 0 {
		return nil
	}
	var one int
	err = s.db.QueryRowContext(ctx, "SELECT 1 FROM api_keys WHERE id = $1 AND user_id = $2", keyID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAPIKeyNotFound
	}
	if err != nil {
		return fmt.Errorf("look up api key: %w", err)
	}
	return nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, key string) (User, APIKey, error) {
	if !s.looksLikeKey(key) {
		return User{}, APIKey{}, ErrAPIKeyInvalid
	}
	hash := hashToken(key)
	var record APIKey
	var user User
	var storedHash string
	var lastUsed, revoked sql.NullTime
	row := s.db.QueryRowContext(ctx,
		`SELECT k.id, k.user_id, k.label, k.prefix, k.created_at, k.last_used_at, k.revoked_at, k.key_hash,
		u.id, u.email, u.is_root, u.created_at, u.updated_at
		FROM api_keys k JOIN users u ON u.id = k.user_id WHERE k.key_hash = $1`, hash)
	err := row.Scan(&record.ID, &record.UserID, &record.Label, &record.Prefix, &record.CreatedAt, &lastUsed, &revoked, &storedHash,
		&user.ID, &user.Email, &user.IsRoot, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, APIKey{}, ErrAPIKeyInvalid
	}
	if err != nil {
		return User{}, APIKey{}, fmt.Errorf("load api key: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hash)) != 1 || revoked.Valid {
		return User{}, APIKey{}, ErrAPIKeyInvalid
	}
	if lastUsed.Valid {
		record.LastUsedAt = &lastUsed.Time
	}
	if err := s.recordKeyUse(ctx, &record); err != nil {
		return User{}, APIKey{}, err
	}
	return user, record, nil
}

func (s *Service) looksLikeKey(key string) bool {
	return len(key) == len(s.keyPrefix)+encodedToken && strings.HasPrefix(key, s.keyPrefix)
}

func (s *Service) recordKeyUse(ctx context.Context, record *APIKey) error {
	now := s.clock()
	if record.LastUsedAt != nil && now.Sub(*record.LastUsedAt) < keyLastUsedInterval {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE api_keys SET last_used_at = $1 WHERE id = $2", now, record.ID); err != nil {
		return fmt.Errorf("record api key use: %w", err)
	}
	record.LastUsedAt = &now
	return nil
}
