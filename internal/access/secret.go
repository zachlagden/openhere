package access

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
)

const (
	secretSettingKey = "internal.viewerSecret"
	secretBytes      = 32
)

func (s *Service) instanceSecret(ctx context.Context) ([]byte, error) {
	s.secretMu.Lock()
	defer s.secretMu.Unlock()
	if s.secret != nil {
		return s.secret, nil
	}
	stored, err := s.loadOrCreateSecret(ctx)
	if err != nil {
		return nil, err
	}
	decoded, err := hex.DecodeString(stored)
	if err != nil || len(decoded) != secretBytes {
		return nil, errors.New("the stored viewer secret is malformed")
	}
	s.secret = decoded
	return decoded, nil
}

func (s *Service) loadOrCreateSecret(ctx context.Context) (string, error) {
	stored, found, err := s.readSecret(ctx)
	if err != nil || found {
		return stored, err
	}
	fresh := make([]byte, secretBytes)
	if _, err := rand.Read(fresh); err != nil {
		return "", wrap("generate viewer secret", err)
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO NOTHING",
		secretSettingKey, hex.EncodeToString(fresh), s.clock())
	if err != nil {
		return "", wrap("store viewer secret", err)
	}
	stored, found, err = s.readSecret(ctx)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("the viewer secret vanished after it was stored")
	}
	return stored, nil
}

func (s *Service) readSecret(ctx context.Context) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM instance_settings WHERE key = $1", secretSettingKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, wrap("read viewer secret", err)
	}
	return value, true, nil
}

func (s *Service) derive(ctx context.Context, purpose string) ([]byte, error) {
	secret, err := s.instanceSecret(ctx)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose))
	return mac.Sum(nil), nil
}

func (s *Service) sign(ctx context.Context, purpose, message string) ([]byte, error) {
	key, err := s.derive(ctx, purpose)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return mac.Sum(nil), nil
}
