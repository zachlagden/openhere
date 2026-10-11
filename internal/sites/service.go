package sites

import (
	"context"
	"log/slog"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/workspaces"
)

const (
	MaxDisplayNameLength = 100
	MaxDescriptionLength = 1000
	MaxFolderNameLength  = 64

	accessPublic = "public"
)

type FileCleaner interface {
	RemoveSite(siteID string) error
}

type Options struct {
	Now           func() time.Time
	SlugGenerator func() (string, error)
	Files         FileCleaner
	Logger        *slog.Logger
	OnSiteChange  func(slug string)

	DefaultAccessMode func(ctx context.Context) (string, error)
	AnalyticsDefault  func(ctx context.Context) (bool, error)
}

type Service struct {
	db       store.Store
	ws       *workspaces.Service
	now      func() time.Time
	generate func() (string, error)
	files    FileCleaner
	logger   *slog.Logger
	changed  func(slug string)

	defaultAccess    func(ctx context.Context) (string, error)
	analyticsDefault func(ctx context.Context) (bool, error)
}

func New(db store.Store, ws *workspaces.Service, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	generate := opts.SlugGenerator
	if generate == nil {
		generate = RandomSlug
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	changed := opts.OnSiteChange
	if changed == nil {
		changed = func(string) {}
	}
	return &Service{db: db, ws: ws, now: now, generate: generate, files: opts.Files, logger: logger, changed: changed, defaultAccess: opts.DefaultAccessMode, analyticsDefault: opts.AnalyticsDefault}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) newSiteAccessMode(ctx context.Context) (string, error) {
	if s.defaultAccess == nil {
		return accessPublic, nil
	}
	return s.defaultAccess(ctx)
}

func (s *Service) newSiteAnalytics(ctx context.Context) (bool, error) {
	if s.analyticsDefault == nil {
		return true, nil
	}
	return s.analyticsDefault(ctx)
}
