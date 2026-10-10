package domains

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/validate"
)

const (
	tokenBytes     = 24
	defaultTimeout = 5 * time.Second
)

type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type Options struct {
	ManagementHost string
	Resolver       TXTResolver
	Now            func() time.Time
	Logger         *slog.Logger
	LookupTimeout  time.Duration
}

type Service struct {
	db       store.Store
	settings *settings.Service
	host     string
	resolver TXTResolver
	now      func() time.Time
	logger   *slog.Logger
	timeout  time.Duration
	cache    *cache
}

func New(db store.Store, settingsSvc *settings.Service, opts Options) *Service {
	resolver := opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	timeout := opts.LookupTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Service{
		db: db, settings: settingsSvc, host: strings.ToLower(opts.ManagementHost), resolver: resolver,
		now: now, logger: logger, timeout: timeout, cache: newCache(now),
	}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) Invalidate() {
	s.cache.invalidate()
}

func newToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (s *Service) checkReserved(name string, cfg settings.Settings) error {
	if underOrEqual(name, s.host) {
		return validate.Invalid("That domain name belongs to this server and cannot be used for a site.")
	}
	if underOrEqual(name, cfg.BaseDomain) {
		return validate.Invalid("Names under the base domain are reserved for site subdomains.")
	}
	return nil
}

func underOrEqual(name, parent string) bool {
	if parent == "" {
		return false
	}
	return name == parent || strings.HasSuffix(name, "."+parent)
}
