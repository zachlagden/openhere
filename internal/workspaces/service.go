package workspaces

import (
	"context"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

type Options struct {
	Now             func() time.Time
	MayCreateShared func(ctx context.Context, userID string) error
}

type Service struct {
	db              store.Store
	now             func() time.Time
	mayCreateShared func(ctx context.Context, userID string) error
}

func New(db store.Store, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, now: now, mayCreateShared: opts.MayCreateShared}
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Service) allowSharedCreation(ctx context.Context, userID string) error {
	if s.mayCreateShared == nil {
		return nil
	}
	return s.mayCreateShared(ctx, userID)
}
