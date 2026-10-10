package domains

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const managementHost = "sites.example.com"

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type dnsAnswer struct {
	records []string
	err     error
}

type fakeDNS struct {
	mu      sync.Mutex
	answers map[string]dnsAnswer
	queries []string
}

func (f *fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, name)
	answer, ok := f.answers[name]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	return answer.records, answer.err
}

func (f *fakeDNS) set(name string, records []string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers[name] = dnsAnswer{records: records, err: err}
}

func (f *fakeDNS) clear(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.answers, name)
}

func (f *fakeDNS) queryCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queries)
}

type env struct {
	t        *testing.T
	db       store.Store
	svc      *Service
	settings *settings.Service
	sites    *sites.Service
	dns      *fakeDNS
	clock    *testClock
	alex     string
	personal string
}

func newEnv(t *testing.T, db store.Store) *env {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	dns := &fakeDNS{answers: map[string]dnsAnswer{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	settingsSvc := settings.New(db, settings.Options{DefaultBaseDomain: managementHost, Now: clock.Now})
	ws := workspaces.New(db, workspaces.Options{Now: clock.Now})
	siteSvc := sites.New(db, ws, sites.Options{Now: clock.Now, Logger: logger})
	alex := storetest.InsertUser(t, db, "alex@example.com")
	personal, err := workspaces.CreatePersonal(context.Background(), db, alex, clock.Now())
	if err != nil {
		t.Fatalf("CreatePersonal: %v", err)
	}
	svc := New(db, settingsSvc, Options{ManagementHost: managementHost, Resolver: dns, Now: clock.Now, Logger: logger})
	return &env{t: t, db: db, svc: svc, settings: settingsSvc, sites: siteSvc, dns: dns, clock: clock, alex: alex, personal: personal.ID}
}

func (e *env) site(slug string) string {
	e.t.Helper()
	site, err := e.sites.CreateSite(context.Background(), sites.CreateInput{UserID: e.alex, Workspace: e.personal, Slug: slug})
	if err != nil {
		e.t.Fatalf("CreateSite %s: %v", slug, err)
	}
	return site.ID
}

func (e *env) claim(siteID, name string) Domain {
	e.t.Helper()
	d, err := e.svc.Claim(context.Background(), siteID, name)
	if err != nil {
		e.t.Fatalf("Claim %s: %v", name, err)
	}
	return d
}

func (e *env) publishChallenge(d Domain) {
	e.dns.set(d.ChallengeName(), []string{d.Token}, nil)
}

func (e *env) verified(siteID, name string) Domain {
	e.t.Helper()
	d := e.claim(siteID, name)
	e.publishChallenge(d)
	got, err := e.svc.Verify(context.Background(), siteID, d.ID)
	if err != nil || !got.Verified() {
		e.t.Fatalf("Verify %s = %+v, %v", name, got, err)
	}
	return got
}

func (e *env) set(raw string) {
	e.t.Helper()
	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		e.t.Fatalf("bad patch: %v", err)
	}
	if _, err := e.settings.Update(context.Background(), patch); err != nil {
		e.t.Fatalf("update settings %s: %v", raw, err)
	}
}

func (e *env) get(siteID, id string) Domain {
	e.t.Helper()
	d, err := e.svc.Get(context.Background(), siteID, id)
	if err != nil {
		e.t.Fatalf("Get: %v", err)
	}
	return d
}

func (e *env) serves(host string) bool {
	e.t.Helper()
	_, ok, err := e.svc.Lookup(context.Background(), host)
	if err != nil {
		e.t.Fatalf("Lookup %s: %v", host, err)
	}
	return ok
}

func each(t *testing.T, fn func(t *testing.T, e *env)) {
	t.Helper()
	storetest.Each(t, func(t *testing.T, db store.Store) {
		fn(t, newEnv(t, db))
	})
}

func transient() error {
	return &net.DNSError{Err: "server misbehaving", Name: "x", IsTemporary: true}
}

func timeout() error {
	return &net.DNSError{Err: "i/o timeout", Name: "x", IsTimeout: true, IsTemporary: true}
}

func isErr(err, target error) bool {
	return errors.Is(err, target)
}

func repeatString(s string, n int) string {
	return strings.Repeat(s, n)
}

func upper(s string) string {
	return strings.ToUpper(s)
}

func notFoundError() net.DNSError {
	return net.DNSError{Err: "no such host", Name: "x", IsNotFound: true}
}
