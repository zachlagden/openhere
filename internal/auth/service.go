package auth

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

type Options struct {
	KeyPrefix string
	Params    Params
	Now       func() time.Time
	Logger    *slog.Logger
}

type Service struct {
	db        store.Store
	keyPrefix string
	params    Params
	now       func() time.Time
	dummyHash string
	logger    *slog.Logger
}

func New(db store.Store, opts Options) (*Service, error) {
	if opts.KeyPrefix == "" {
		return nil, fmt.Errorf("new auth service: key prefix is empty")
	}
	params := opts.Params
	if params == (Params{}) {
		params = DefaultParams
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	dummy, err := HashPassword("timing-equaliser-not-a-real-password", params)
	if err != nil {
		return nil, fmt.Errorf("new auth service: %w", err)
	}
	return &Service{db: db, keyPrefix: opts.KeyPrefix, params: params, now: now, dummyHash: dummy, logger: logger}, nil
}

func (s *Service) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}
