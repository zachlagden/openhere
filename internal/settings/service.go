package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/validate"
)

type Options struct {
	Now func() time.Time
}

type Service struct {
	db  store.Store
	now func() time.Time

	mu         sync.Mutex
	cached     *Settings
	generation uint64
}

func New(db store.Store, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, now: now}
}

func Defaults() Settings {
	var settings Settings
	for _, def := range definitions {
		if err := def.apply(&settings, def.fallback); err != nil {
			panic(fmt.Sprintf("default for %s is invalid: %v", def.key, err))
		}
	}
	return settings
}

func Values(settings Settings) map[string]any {
	values := make(map[string]any, len(definitions))
	for _, def := range definitions {
		values[def.key] = def.value(settings)
	}
	return values
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	s.mu.Lock()
	if s.cached != nil {
		cached := *s.cached
		s.mu.Unlock()
		return cached, nil
	}
	generation := s.generation
	s.mu.Unlock()

	loaded, err := s.load(ctx)
	if err != nil {
		return Settings{}, err
	}
	s.mu.Lock()
	if s.generation == generation {
		s.cached = &loaded
	}
	s.mu.Unlock()
	return loaded, nil
}

func (s *Service) load(ctx context.Context) (Settings, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM instance_settings")
	if err != nil {
		return Settings{}, fmt.Errorf("load settings: %w", err)
	}
	stored := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return Settings{}, closeAfter(rows, fmt.Errorf("scan setting: %w", err))
		}
		stored[key] = value
	}
	if err := rows.Err(); err != nil {
		return Settings{}, closeAfter(rows, fmt.Errorf("read settings: %w", err))
	}
	if err := rows.Close(); err != nil {
		return Settings{}, fmt.Errorf("close setting rows: %w", err)
	}
	return build(stored)
}

func build(stored map[string]string) (Settings, error) {
	var settings Settings
	for _, def := range definitions {
		value, ok := stored[def.key]
		if !ok {
			value = def.fallback
		}
		if err := def.apply(&settings, value); err != nil {
			return Settings{}, err
		}
	}
	return settings, nil
}

func (s *Service) Update(ctx context.Context, patch map[string]json.RawMessage) (Settings, error) {
	if len(patch) == 0 {
		return Settings{}, validate.Invalid("Send at least one setting to change.")
	}
	changes := make(map[string]string, len(patch))
	for key, raw := range patch {
		def, ok := findDefinition(key)
		if !ok {
			return Settings{}, validate.Invalid(fmt.Sprintf("Unknown setting %q.", key))
		}
		stored, err := def.decode(raw)
		if err != nil {
			return Settings{}, err
		}
		changes[key] = stored
	}
	err := store.WithTx(ctx, s.db, func(q store.Querier) error { return s.write(ctx, q, changes) })
	s.invalidate()
	if err != nil {
		return Settings{}, err
	}
	return s.Get(ctx)
}

func (s *Service) write(ctx context.Context, q store.Querier, changes map[string]string) error {
	now := s.now().UTC().Truncate(time.Second)
	for key, value := range changes {
		_, err := q.ExecContext(ctx,
			`INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			key, value, now)
		if err != nil {
			return fmt.Errorf("write setting %s: %w", key, err)
		}
	}
	return nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.generation++
	s.mu.Unlock()
}
