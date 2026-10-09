package publish

import "sync"

type lockEntry struct {
	mu   sync.RWMutex
	refs int
}

type keyedLocker struct {
	mu      sync.Mutex
	entries map[string]*lockEntry
}

func newKeyedLocker() keyedLocker {
	return keyedLocker{entries: map[string]*lockEntry{}}
}

func (l *keyedLocker) acquire(key string) *lockEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok {
		entry = &lockEntry{}
		l.entries[key] = entry
	}
	entry.refs++
	return entry
}

func (l *keyedLocker) release(key string, entry *lockEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, key)
	}
}

func (l *keyedLocker) lock(key string) func() {
	entry := l.acquire(key)
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.release(key, entry)
	}
}

func (l *keyedLocker) rlock(key string) func() {
	entry := l.acquire(key)
	entry.mu.RLock()
	return func() {
		entry.mu.RUnlock()
		l.release(key, entry)
	}
}
