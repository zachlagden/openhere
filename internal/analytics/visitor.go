package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/store"
)

const (
	secretSettingKey = "internal.analyticsSecret"
	secretBytes      = 32
	visitorBytes     = 16
	DayLayout        = "2006-01-02"
)

type Hasher struct {
	db     store.Store
	box    *secrets.Box
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	secret []byte
}

func NewHasher(db store.Store, box *secrets.Box, logger *slog.Logger) *Hasher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hasher{db: db, box: box, logger: logger, now: time.Now}
}

func (h *Hasher) Visitor(ctx context.Context, addr netip.Addr, userAgent, siteID string, at time.Time) (string, error) {
	secret, err := h.loadSecret(ctx)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	for _, part := range []string{proxy.LimiterKey(addr), userAgent, siteID, at.UTC().Format(DayLayout)} {
		mac.Write([]byte(part))
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil)[:visitorBytes]), nil
}

func (h *Hasher) loadSecret(ctx context.Context) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.secret != nil {
		return h.secret, nil
	}
	secret, err := h.readOrCreate(ctx)
	if err != nil {
		return nil, err
	}
	h.secret = secret
	return secret, nil
}

func (h *Hasher) readOrCreate(ctx context.Context) ([]byte, error) {
	sealed, found, err := h.readSealed(ctx)
	if err != nil {
		return nil, err
	}
	if found {
		if secret, ok := h.open(sealed); ok {
			return secret, nil
		}
		h.logger.Warn("the stored analytics secret cannot be opened, so a new one replaces it")
		return h.replace(ctx)
	}
	return h.create(ctx)
}

func (h *Hasher) open(sealed string) ([]byte, bool) {
	plain, err := h.box.Open(sealed)
	if err != nil {
		return nil, false
	}
	decoded, err := hex.DecodeString(plain)
	if err != nil || len(decoded) != secretBytes {
		return nil, false
	}
	return decoded, true
}

func (h *Hasher) create(ctx context.Context) ([]byte, error) {
	sealed, fresh, err := h.fresh()
	if err != nil {
		return nil, err
	}
	_, err = h.db.ExecContext(ctx,
		"INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO NOTHING",
		secretSettingKey, sealed, h.stamp())
	if err != nil {
		return nil, fmt.Errorf("store analytics secret: %w", err)
	}
	stored, found, err := h.readSealed(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("the analytics secret vanished after it was stored")
	}
	if secret, ok := h.open(stored); ok {
		return secret, nil
	}
	return fresh, nil
}

func (h *Hasher) replace(ctx context.Context) ([]byte, error) {
	sealed, fresh, err := h.fresh()
	if err != nil {
		return nil, err
	}
	_, err = h.db.ExecContext(ctx,
		`INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		secretSettingKey, sealed, h.stamp())
	if err != nil {
		return nil, fmt.Errorf("replace analytics secret: %w", err)
	}
	return fresh, nil
}

func (h *Hasher) fresh() (string, []byte, error) {
	fresh := make([]byte, secretBytes)
	if _, err := rand.Read(fresh); err != nil {
		return "", nil, fmt.Errorf("generate analytics secret: %w", err)
	}
	sealed, err := h.box.Seal(hex.EncodeToString(fresh))
	if err != nil {
		return "", nil, fmt.Errorf("seal analytics secret: %w", err)
	}
	return sealed, fresh, nil
}

func (h *Hasher) readSealed(ctx context.Context) (string, bool, error) {
	var value string
	err := h.db.QueryRowContext(ctx, "SELECT value FROM instance_settings WHERE key = $1", secretSettingKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read analytics secret: %w", err)
	}
	return value, true, nil
}

func (h *Hasher) stamp() time.Time {
	return h.now().UTC().Truncate(time.Second)
}
