package access

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/store"
)

type Options struct {
	Mailer mail.Sender
	Params auth.Params
	Now    func() time.Time
	Logger *slog.Logger
	Spawn  func(task func())
}

type Service struct {
	db     store.Store
	mailer mail.Sender
	params auth.Params
	now    func() time.Time
	logger *slog.Logger
	spawn  func(task func())

	secretMu sync.Mutex
	secret   []byte

	dummyOnce sync.Once
	dummyHash string
	dummyErr  error
}

func New(db store.Store, opts Options) *Service {
	mailer := opts.Mailer
	if mailer == nil {
		mailer = mail.Unconfigured{}
	}
	params := opts.Params
	if params == (auth.Params{}) {
		params = auth.DefaultParams
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
	return &Service{db: db, mailer: mailer, params: params, now: now, logger: logger, spawn: spawn}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) EmailCodeAvailable() bool {
	return s.mailer.Configured()
}

func (s *Service) Purge(ctx context.Context) error {
	now := s.clock()
	if _, err := s.db.ExecContext(ctx, "DELETE FROM viewer_codes WHERE expires_at <= $1", now); err != nil {
		return wrap("purge expired codes", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM viewer_handoffs WHERE expires_at <= $1", now); err != nil {
		return wrap("purge expired handoffs", err)
	}
	return nil
}
