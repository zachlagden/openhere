package certs

import (
	"log/slog"
	"sync"
	"time"
)

const refusalLogInterval = time.Minute

type refusalCounter struct {
	logger *slog.Logger
	now    func() time.Time

	mu       sync.Mutex
	count    int
	lastEmit time.Time
}

func newRefusalCounter(logger *slog.Logger, now func() time.Time) *refusalCounter {
	return &refusalCounter{logger: logger, now: now, lastEmit: now()}
}

func (c *refusalCounter) note(reason string) {
	c.mu.Lock()
	c.count++
	if c.now().Sub(c.lastEmit) < refusalLogInterval {
		c.mu.Unlock()
		return
	}
	count := c.count
	c.count = 0
	c.lastEmit = c.now()
	c.mu.Unlock()
	c.logger.Debug("refused on-demand certificate requests", "count", count, "latest_reason", reason)
}
