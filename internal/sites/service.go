package sites

import (
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	MaxDisplayNameLength = 100
	MaxDescriptionLength = 1000
	MaxFolderNameLength  = 64
)

type Options struct {
	Now           func() time.Time
	SlugGenerator func() (string, error)
}

type Service struct {
	db       store.Store
	ws       *workspaces.Service
	now      func() time.Time
	generate func() (string, error)
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
	return &Service{db: db, ws: ws, now: now, generate: generate}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}
