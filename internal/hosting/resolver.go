package hosting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

const (
	foundTTL      = time.Minute
	missingTTL    = 10 * time.Second
	maxCachedKeys = 4096
)

type Site struct {
	ID        string
	Slug      string
	VersionID string
	SPAMode   bool
}

type cacheEntry struct {
	site      Site
	live      bool
	expiresAt time.Time
}

type Resolver struct {
	db  store.Querier
	now func() time.Time

	mu         sync.Mutex
	entries    map[string]cacheEntry
	generation uint64
}

func NewResolver(db store.Querier, now func() time.Time) *Resolver {
	if now == nil {
		now = time.Now
	}
	return &Resolver{db: db, now: now, entries: map[string]cacheEntry{}}
}

func (r *Resolver) Lookup(ctx context.Context, slug string) (Site, bool, error) {
	if entry, ok := r.cached(slug); ok {
		return entry.site, entry.live, nil
	}
	r.mu.Lock()
	generation := r.generation
	r.mu.Unlock()

	site, live, err := r.load(ctx, slug)
	if err != nil {
		return Site{}, false, err
	}
	r.store(slug, site, live, generation)
	return site, live, nil
}

func (r *Resolver) Invalidate(slug string) {
	r.mu.Lock()
	delete(r.entries, slug)
	r.generation++
	r.mu.Unlock()
}

func (r *Resolver) cached(slug string) (cacheEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[slug]
	if !ok || !r.now().Before(entry.expiresAt) {
		return cacheEntry{}, false
	}
	return entry, true
}

func (r *Resolver) store(slug string, site Site, live bool, generation uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.generation != generation {
		return
	}
	now := r.now()
	if len(r.entries) >= maxCachedKeys {
		r.makeRoom(now)
	}
	ttl := missingTTL
	if live {
		ttl = foundTTL
	}
	r.entries[slug] = cacheEntry{site: site, live: live, expiresAt: now.Add(ttl)}
}

func (r *Resolver) makeRoom(now time.Time) {
	for key, entry := range r.entries {
		if !now.Before(entry.expiresAt) {
			delete(r.entries, key)
		}
	}
	for key := range r.entries {
		if len(r.entries) < maxCachedKeys {
			return
		}
		delete(r.entries, key)
	}
}

func (r *Resolver) load(ctx context.Context, slug string) (Site, bool, error) {
	var site Site
	var version sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT id, current_version_id, spa_mode FROM sites WHERE slug = $1", slug).
		Scan(&site.ID, &version, &site.SPAMode)
	if errors.Is(err, sql.ErrNoRows) {
		return Site{}, false, nil
	}
	if err != nil {
		return Site{}, false, fmt.Errorf("look up site: %w", err)
	}
	site.Slug = slug
	site.VersionID = version.String
	return site, version.Valid, nil
}
