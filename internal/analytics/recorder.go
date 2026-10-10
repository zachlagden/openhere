package analytics

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
)

const (
	DefaultBufferSize    = 4096
	DefaultBatchSize     = 200
	DefaultFlushInterval = 5 * time.Second
	insertTimeout        = 30 * time.Second
	maxCityLength        = 100
)

type Locator interface {
	Locate(addr netip.Addr) (country, city string)
}

type RecorderOptions struct {
	Hasher        *Hasher
	Locator       Locator
	Logger        *slog.Logger
	Now           func() time.Time
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

type Recorder struct {
	db      store.Store
	hasher  *Hasher
	locator Locator
	logger  *slog.Logger
	now     func() time.Time
	batch   int
	every   time.Duration

	mu      sync.RWMutex
	closed  bool
	hits    chan Hit
	flushes chan chan struct{}
	done    chan struct{}
	dropped atomic.Int64
}

func NewRecorder(db store.Store, opts RecorderOptions) *Recorder {
	r := &Recorder{
		db: db, hasher: opts.Hasher, locator: opts.Locator, logger: opts.Logger, now: opts.Now,
		batch: opts.BatchSize, every: opts.FlushInterval,
		flushes: make(chan chan struct{}), done: make(chan struct{}),
	}
	if r.logger == nil {
		r.logger = slog.Default()
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.batch <= 0 {
		r.batch = DefaultBatchSize
	}
	if r.every <= 0 {
		r.every = DefaultFlushInterval
	}
	size := opts.BufferSize
	if size <= 0 {
		size = DefaultBufferSize
	}
	r.hits = make(chan Hit, size)
	return r
}

func (r *Recorder) Start() {
	go r.run()
}

func (r *Recorder) Record(hit Hit) {
	if _, counted := statusClass(hit.Status); !counted {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		r.dropped.Add(1)
		return
	}
	if hit.At.IsZero() {
		hit.At = r.now()
	}
	select {
	case r.hits <- hit:
	default:
		r.dropped.Add(1)
	}
}

func (r *Recorder) Dropped() int64 {
	return r.dropped.Load()
}

func (r *Recorder) Flush(ctx context.Context) error {
	ack := make(chan struct{})
	select {
	case r.flushes <- ack:
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-ack:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Recorder) Close(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	close(r.hits)
	r.mu.Unlock()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Recorder) run() {
	defer close(r.done)
	ticker := time.NewTicker(r.every)
	defer ticker.Stop()
	pending := make([]Event, 0, r.batch)
	for {
		select {
		case hit, open := <-r.hits:
			if !open {
				r.write(pending)
				return
			}
			pending = r.add(pending, hit)
			if len(pending) >= r.batch {
				r.write(pending)
				pending = pending[:0]
			}
		case <-ticker.C:
			r.write(pending)
			pending = pending[:0]
		case ack := <-r.flushes:
			pending = r.drain(pending)
			r.write(pending)
			pending = pending[:0]
			close(ack)
		}
	}
}

func (r *Recorder) drain(pending []Event) []Event {
	for {
		select {
		case hit, open := <-r.hits:
			if !open {
				return pending
			}
			pending = r.add(pending, hit)
		default:
			return pending
		}
	}
}

func (r *Recorder) add(pending []Event, hit Hit) []Event {
	event, ok := r.build(hit)
	if !ok {
		r.dropped.Add(1)
		return pending
	}
	return append(pending, event)
}

func (r *Recorder) write(events []Event) {
	if len(events) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), insertTimeout)
	defer cancel()
	for start := 0; start < len(events); start += r.batch {
		end := min(start+r.batch, len(events))
		r.insert(ctx, events[start:end])
	}
}

func (r *Recorder) insert(ctx context.Context, events []Event) {
	err := insertEvents(ctx, r.db, events)
	if err == nil {
		return
	}
	r.logger.Warn("insert analytics batch failed, retrying one row at a time", "rows", len(events), "error", err)
	for _, event := range events {
		if err := insertEvents(ctx, r.db, []Event{event}); err != nil {
			r.dropped.Add(1)
			r.logger.Warn("dropped an analytics event", "error", err)
		}
	}
}
