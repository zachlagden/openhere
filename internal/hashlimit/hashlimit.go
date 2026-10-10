package hashlimit

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	DefaultConcurrency = 2
	MinConcurrency     = 1
	MaxConcurrency     = 16
	DefaultWait        = 5 * time.Second
)

var ErrBusy = errors.New("password hashing is busy")

type Limiter struct {
	slots chan struct{}
	wait  time.Duration
}

func New(concurrency int, wait time.Duration) *Limiter {
	if concurrency < MinConcurrency {
		concurrency = MinConcurrency
	}
	return &Limiter{slots: make(chan struct{}, concurrency), wait: wait}
}

func (l *Limiter) Acquire(ctx context.Context) (func(), error) {
	select {
	case l.slots <- struct{}{}:
		return l.release, nil
	default:
	}
	timer := time.NewTimer(l.wait)
	defer timer.Stop()
	select {
	case l.slots <- struct{}{}:
		return l.release, nil
	case <-timer.C:
		return nil, ErrBusy
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %w", ErrBusy, ctx.Err())
	}
}

func (l *Limiter) release() {
	<-l.slots
}

func (l *Limiter) RetryAfter() time.Duration {
	return l.wait
}

var current atomic.Pointer[Limiter]

func init() {
	current.Store(New(DefaultConcurrency, DefaultWait))
}

func SetDefault(l *Limiter) {
	current.Store(l)
}

func Default() *Limiter {
	return current.Load()
}

func Acquire(ctx context.Context) (func(), error) {
	return Default().Acquire(ctx)
}
