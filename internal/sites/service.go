package sites

import (
	"log/slog"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	MaxDisplayNameLength = 100
	MaxDescriptionLength = 1000
	MaxFolderNameLength  = 64
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
}

type Service struct {
	db       store.Store
	ws       *workspaces.Service
	now      func() time.Time
	generate func() (string, error)
	files    FileCleaner
	logger   *slog.Logger
	changed  func(slug string)
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
	return &Service{db: db, ws: ws, now: now, generate: generate, files: opts.Files, logger: logger, changed: changed}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}
