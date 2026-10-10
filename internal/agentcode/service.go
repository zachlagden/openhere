package agentcode

import (
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/store"
)

const (
	CodeDigits   = 8
	CodeAttempts = 5
	CodeLifetime = 10 * time.Minute

	DefaultLabel = "agent"
	sendTimeout  = 30 * time.Second
	signPurpose  = "agent-code"
)

var (
	ErrEmailNotConfigured = errors.New("this instance cannot send email")
	ErrCodeInvalid        = errors.New("the code is wrong, expired or already used")
)

type Options struct {
	BaseURL string
	Mailer  mail.Sender
	Now     func() time.Time
	Logger  *slog.Logger
	Spawn   func(task func())
}

type Service struct {
	db      store.Store
	auth    *auth.Service
	secrets *secrets.Box
	host    string
	mailer  mail.Sender
	now     func() time.Time
	logger  *slog.Logger
	spawn   func(task func())
}

func New(db store.Store, authSvc *auth.Service, box *secrets.Box, opts Options) *Service {
	mailer := opts.Mailer
	if mailer == nil {
		mailer = mail.Unconfigured{}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	spawn := opts.Spawn
	if spawn == nil {
		spawn = func(task func()) { go task() }
	}
	return &Service{
		db: db, auth: authSvc, secrets: box, host: hostOf(opts.BaseURL),
		mailer: mailer, now: now, logger: logger, spawn: spawn,
	}
}

func hostOf(baseURL string) string {
	trimmed := strings.TrimSuffix(baseURL, "/")
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return trimmed
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) Available() bool {
	return s.mailer.Configured()
}
