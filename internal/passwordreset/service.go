package passwordreset

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/store"
)

const (
	Lifetime    = time.Hour
	sendTimeout = 30 * time.Second
)

var ErrInvalidToken = errors.New("the reset link is unknown, expired or already used")

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
	baseURL string
	mailer  mail.Sender
	now     func() time.Time
	logger  *slog.Logger
	spawn   func(task func())
}

func New(db store.Store, authSvc *auth.Service, opts Options) *Service {
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
		db: db, auth: authSvc, baseURL: strings.TrimSuffix(opts.BaseURL, "/"),
		mailer: mailer, now: now, logger: logger, spawn: spawn,
	}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) linkFor(token string) string {
	return s.baseURL + "/reset-password/" + token
}
