package publish

import (
	"log/slog"
	"sync"
	"time"

	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	PendingTTL    = 24 * time.Hour
	maxListedRows = 200
)

type Options struct {
	Now          func() time.Time
	Logger       *slog.Logger
	OnSiteChange func(slug string)
}

type Service struct {
	db       store.Store
	sites    *sites.Service
	ws       *workspaces.Service
	settings *settings.Service
	files    *sitefiles.Store
	now      func() time.Time
	logger   *slog.Logger
	locks    keyedLocker

	pendingMu sync.Mutex
	changed   func(slug string)
}

func New(db store.Store, siteSvc *sites.Service, ws *workspaces.Service, cfg *settings.Service, files *sitefiles.Store, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	changed := opts.OnSiteChange
	if changed == nil {
		changed = func(string) {}
	}
	return &Service{db: db, sites: siteSvc, ws: ws, settings: cfg, files: files, now: now, logger: logger, locks: newKeyedLocker(), changed: changed}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Microsecond)
}
