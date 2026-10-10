package analytics

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

type fakeLocator struct {
	country string
	city    string
}

func (f fakeLocator) Locate(netip.Addr) (string, string) { return f.country, f.city }

type env struct {
	t        *testing.T
	db       store.Store
	clock    *testClock
	settings *settings.Service
	ws       *workspaces.Service
	sites    *sites.Service
	svc      *Service
	hasher   *Hasher
	box      *secrets.Box
	alex     string
	sam      string
	acme     workspaces.Workspace
}

func newEnv(t *testing.T, db store.Store) *env {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)}
	box, err := secrets.New(bytes.Repeat([]byte{7}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, db: db, clock: clock, box: box}
	e.settings = settings.New(db, settings.Options{Now: clock.Now, Secrets: box})
	e.ws = workspaces.New(db, workspaces.Options{Now: clock.Now})
	e.sites = sites.New(db, e.ws, sites.Options{Now: clock.Now})
	e.hasher = NewHasher(db, box, discardLogger())
	e.svc = New(db, e.sites, e.ws, e.settings, Options{Now: clock.Now})
	e.alex = e.user("alex@example.com")
	e.sam = e.user("sam@example.com")
	acc, err := e.ws.CreateShared(context.Background(), e.alex, "Acme Ltd", "acme")
	if err != nil {
		t.Fatal(err)
	}
	e.acme = acc.Workspace
	return e
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (e *env) user(email string) string {
	e.t.Helper()
	id := storetest.InsertUser(e.t, e.db, email)
	if _, err := workspaces.CreatePersonal(context.Background(), e.db, id, e.clock.Now()); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) site(slug string) sites.Site {
	e.t.Helper()
	site, err := e.sites.CreateSite(context.Background(), sites.CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: slug})
	if err != nil {
		e.t.Fatal(err)
	}
	return site
}

func (e *env) insert(events ...Event) {
	e.t.Helper()
	if err := insertEvents(context.Background(), e.db, events); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) recorder(opts RecorderOptions) *Recorder {
	e.t.Helper()
	if opts.Hasher == nil {
		opts.Hasher = e.hasher
	}
	opts.Logger = discardLogger()
	opts.Now = e.clock.Now
	r := NewRecorder(e.db, opts)
	r.Start()
	e.t.Cleanup(func() {
		if err := r.Close(context.Background()); err != nil {
			e.t.Errorf("close recorder: %v", err)
		}
	})
	return r
}

func (e *env) report(siteSlug string, p Params) Report {
	e.t.Helper()
	report, err := e.svc.SiteReport(context.Background(), e.alex, siteSlug, p)
	if err != nil {
		e.t.Fatal(err)
	}
	return report
}

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func view(siteID, when, path, visitor string) Event {
	return Event{
		SiteID: siteID, OccurredAt: at(when), Path: path, StatusClass: StatusSuccess,
		Browser: "Firefox", OS: "Linux", Device: DeviceDesktop, Visitor: visitor,
	}
}

func rangeParams(from, to string) Params {
	return Params{From: day(from), To: day(to), Interval: IntervalDay, Top: 10}
}

func valuesOf(list []Ranked) map[string]int64 {
	out := map[string]int64{}
	for _, r := range list {
		out[r.Value] = r.Views
	}
	return out
}
