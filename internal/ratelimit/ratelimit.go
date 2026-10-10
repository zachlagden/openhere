package ratelimit

import (
	"sync"
	"time"
)

const maxTrackedKeys = 50000

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
}

func New(max int, period time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{max: max, period: period, now: now, windows: make(map[string]*window)}
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
	if len(l.windows) < maxTrackedKeys {
		return
	}
	for key, w := range l.windows {
		if !now.Before(w.resetAt) {
			delete(l.windows, key)
		}
	}
	for key := range l.windows {
		if len(l.windows) < maxTrackedKeys {
			return
		}
		delete(l.windows, key)
	}
}
