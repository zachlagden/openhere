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
}

type Service struct {
	db       store.Store
	ws       *workspaces.Service
	now      func() time.Time
	generate func() (string, error)
	files    FileCleaner
	logger   *slog.Logger
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
	return &Service{db: db, ws: ws, now: now, generate: generate, files: opts.Files, logger: logger}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}
