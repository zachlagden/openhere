package ratelimit

import (
	"slices"
	"sync"
	"time"
)

const (
	maxTrackedKeys = 50000
	evictionShare  = 20
)

type Rule struct {
	Max    int
	Window time.Duration
}

type window struct {
	count   int
	resetAt time.Time
}

type Limiter struct {
	mu      sync.Mutex
	max     int
	period  time.Duration
	now     func() time.Time
	windows map[string]*window
	maxKeys int
}

func New(max int, period time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{max: max, period: period, now: now, windows: make(map[string]*window), maxKeys: maxTrackedKeys}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.windows[key]
	if !ok || !now.Before(w.resetAt) {
		l.makeRoom(now)
		w = &window{resetAt: now.Add(l.period)}
		l.windows[key] = w
	}
	if w.count >= l.max {
		return false, w.resetAt.Sub(now)
	}
	w.count++
	return true, 0
}

func (l *Limiter) Count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.windows[key]
	if !ok || !l.now().Before(w.resetAt) {
		return 0
	}
	return w.count
}

func (l *Limiter) Full(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.windows[key]
	if !ok || !now.Before(w.resetAt) || w.count < l.max {
		return false, 0
	}
	return true, w.resetAt.Sub(now)
}

func (l *Limiter) Record(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.windows[key]
	if !ok || !now.Before(w.resetAt) {
		l.makeRoom(now)
		w = &window{resetAt: now.Add(l.period)}
		l.windows[key] = w
	}
	w.count++
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.windows, key)
}

func (l *Limiter) makeRoom(now time.Time) {
	if len(l.windows) < l.maxKeys {
		return
	}
	for key, w := range l.windows {
		if !now.Before(w.resetAt) {
			delete(l.windows, key)
		}
	}
	target := l.maxKeys - max(1, l.maxKeys/evictionShare)
	if len(l.windows) <= target {
		return
	}
	entries := make([]evictable, 0, len(l.windows))
	for key, w := range l.windows {
		entries = append(entries, evictable{key: key, resetAt: w.resetAt})
	}
	slices.SortFunc(entries, func(a, b evictable) int { return a.resetAt.Compare(b.resetAt) })
	for _, entry := range entries[:len(l.windows)-target] {
		delete(l.windows, entry.key)
	}
}

type evictable struct {
	key     string
	resetAt time.Time
}
