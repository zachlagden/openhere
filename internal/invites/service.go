package invites

import (
	"log/slog"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type Options struct {
	BaseURL string
	Mailer  mail.Sender
	Now     func() time.Time
	Logger  *slog.Logger
}

type Service struct {
	db         store.Store
	auth       *auth.Service
	workspaces *workspaces.Service
	settings   *settings.Service
	baseURL    string
	mailer     mail.Sender
	now        func() time.Time
	logger     *slog.Logger
}

func New(db store.Store, authSvc *auth.Service, ws *workspaces.Service, settingsSvc *settings.Service, opts Options) *Service {
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
	return &Service{
		db: db, auth: authSvc, workspaces: ws, settings: settingsSvc,
		baseURL: strings.TrimSuffix(opts.BaseURL, "/"), mailer: mailer, now: now, logger: logger,
	}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) EmailAvailable() bool {
	return s.mailer.Configured()
}

func (s *Service) linkFor(token string) string {
	return s.baseURL + "/invite/" + token
}
