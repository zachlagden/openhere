package hosting

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

var fastParams = auth.Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8}

const (
	managementHost = "sites.example.com"
	siteHost       = "demo.sites.example.com"
)

type env struct {
	t          *testing.T
	db         store.Store
	dataDir    string
	settings   *settings.Service
	publish    *publish.Service
	sites      *sites.Service
	workspaces *workspaces.Service
	files      *sitefiles.Store
	resolver   *Resolver
	probes     *Probes
	router     *Router
	access     *access.Service
	domains    *domains.Service
	dns        *fakeDNS
	mailer     *mail.Recorder
	clock      *testClock
	views      *viewLog
	alex       string
	managed    atomic.Int64
}

type siteSpec struct {
	files map[string]string
	types map[string]string
	spa   bool
}

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

type envOptions struct {
	public  config.PublicURL
	limits  *GateLimits
	proxies proxy.Trust
	hsts    bool
}

func newEnv(t *testing.T, db store.Store) *env {
	t.Helper()
	return newEnvWith(t, db, envOptions{})
}

func newEnvWith(t *testing.T, db store.Store, opts envOptions) *env {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dataDir := t.TempDir()
	files, err := sitefiles.New(dataDir)
	if err != nil {
		t.Fatalf("sitefiles.New: %v", err)
	}
	e := &env{t: t, db: db, dataDir: dataDir, files: files}
	e.clock = &testClock{now: time.Now().UTC().Truncate(time.Second)}
	e.mailer = mail.NewRecorder()
	e.views = &viewLog{}
	e.access = access.New(db, access.Options{
		Mailer: e.mailer, Params: fastParams, Now: e.clock.Now, Logger: logger, Spawn: func(task func()) { task() },
	})
	e.resolver = NewResolver(db, nil)
	e.settings = settings.New(db, settings.Options{DefaultBaseDomain: managementHost})
	ws := workspaces.New(db, workspaces.Options{})
	e.dns = &fakeDNS{answers: map[string][]string{}}
	e.domains = domains.New(db, e.settings, domains.Options{ManagementHost: managementHost, Resolver: e.dns, Now: e.clock.Now, Logger: logger})
	changed := func(slug string) {
		e.resolver.Invalidate(slug)
		e.domains.Invalidate()
	}
	siteSvc := sites.New(db, ws, sites.Options{Files: files, Logger: logger, OnSiteChange: changed})
	e.sites = siteSvc
	e.workspaces = ws
	e.publish = publish.New(db, siteSvc, ws, e.settings, files, publish.Options{Logger: logger, OnSiteChange: changed})
	e.alex = storetest.InsertUser(t, db, "alex@example.com")
	if _, err := workspaces.CreatePersonal(context.Background(), db, e.alex, timeNow()); err != nil {
		t.Fatalf("CreatePersonal: %v", err)
	}
	e.probes = NewProbes(nil)
	management := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.managed.Add(1)
		w.Header().Set("X-Management", "1")
		w.Header().Set("X-Management-Path", r.URL.Path)
		writeText(w, http.StatusOK, "management", true)
	})
	public := opts.public
	if public == (config.PublicURL{}) {
		public = config.PublicURL{Scheme: "https", Host: managementHost}
	}
	e.router = New(management, e.settings, db, files, Options{
		Public: public, Probes: e.probes, Resolver: e.resolver, Logger: logger, Access: e.access, Proxies: opts.proxies,
		Limits: opts.limits, Now: e.clock.Now, Domains: e.domains, HSTS: opts.hsts, Views: e.views,
	})
	return e
}

func (e *env) set(raw string) {
	e.t.Helper()
	if _, err := e.settings.Update(context.Background(), parsePatch(e.t, raw)); err != nil {
		e.t.Fatalf("update settings %s: %v", raw, err)
	}
}

func (e *env) enableSubdomains() {
	e.t.Helper()
	if _, err := e.settings.RecordWildcard(context.Background(), true, ""); err != nil {
		e.t.Fatal(err)
	}
	e.set(`{"hosting.subdomainEnabled": true}`)
}

func hashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (e *env) specs(in siteSpec) []publish.FileSpec {
	paths := make([]string, 0, len(in.files))
	for p := range in.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]publish.FileSpec, 0, len(paths))
	for _, p := range paths {
		out = append(out, publish.FileSpec{Path: p, Size: int64(len(in.files[p])), SHA256: hashOf(in.files[p]), ContentType: in.types[p]})
	}
	return out
}

func (e *env) publishSite(slug string, in siteSpec) (siteID, versionID string) {
	e.t.Helper()
	return e.publishSiteIn(slug, "", in)
}

func (e *env) publishSiteIn(slug, workspace string, in siteSpec) (siteID, versionID string) {
	e.t.Helper()
	ctx := context.Background()
	var pending publish.Pending
	var err error
	if _, lookupErr := e.publish.List(ctx, e.alex, slug); lookupErr != nil {
		pending, err = e.publish.Begin(ctx, publish.BeginInput{UserID: e.alex, Workspace: workspace, Slug: slug, SpaMode: in.spa, Files: e.specs(in)})
	} else {
		pending, err = e.publish.NewVersion(ctx, publish.NewVersionInput{UserID: e.alex, Site: slug, Files: e.specs(in)})
	}
	if err != nil {
		e.t.Fatalf("publish %s: %v", slug, err)
	}
	for _, spec := range pending.Files {
		if _, err := e.publish.Upload(ctx, e.alex, slug, pending.Version.ID, spec.Path, strings.NewReader(in.files[spec.Path])); err != nil {
			e.t.Fatalf("upload %s: %v", spec.Path, err)
		}
	}
	if _, err := e.publish.Finalize(ctx, e.alex, slug, pending.Version.ID); err != nil {
		e.t.Fatalf("finalize %s: %v", slug, err)
	}
	return pending.Site.ID, pending.Version.ID
}

func (e *env) versionDir(siteID, versionID string) string {
	return filepath.Join(e.dataDir, "sites", siteID, "versions", versionID, "files")
}

func (e *env) request(method, host, target string, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.t.Context(), method, "http://"+host+target, nil)
	req.Host = host
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *env) get(host, target string, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.request(http.MethodGet, host, target, headers...)
}

func (e *env) site(target string, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.get(siteHost, target, headers...)
}

type snapshot struct {
	status  int
	headers string
	body    string
}

func capture(rec *httptest.ResponseRecorder) snapshot {
	var lines []string
	for name, values := range rec.Result().Header {
		lines = append(lines, name+": "+strings.Join(values, ","))
	}
	sort.Strings(lines)
	return snapshot{status: rec.Code, headers: strings.Join(lines, "\n"), body: rec.Body.String()}
}

func expectBody(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if rec.Code != status || rec.Body.String() != body {
		t.Fatalf("response = %d %q, want %d %q", rec.Code, rec.Body.String(), status, body)
	}
}

func writeThroughSymlink(t *testing.T, dir, name, target string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.Remove(full); err != nil {
		t.Fatalf("remove %s: %v", name, err)
	}
	if err := os.Symlink(target, full); err != nil {
		t.Fatalf("symlink %s: %v", name, err)
	}
}

func timeNow() time.Time {
	return time.Now().UTC()
}

func parsePatch(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad patch %s: %v", raw, err)
	}
	return out
}

func beginDraft(e *env, slug string) publish.BeginInput {
	return publish.BeginInput{
		UserID: e.alex, Slug: slug,
		Files: []publish.FileSpec{{Path: "index.html", Size: 1, SHA256: hashOf("x")}},
	}
}

func (e *env) rawGet(host, target string) (*httptest.ResponseRecorder, bool) {
	e.t.Helper()
	wire := "GET " + target + " HTTP/1.1\r\nHost: " + host + "\r\n\r\n"
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(wire)))
	if err != nil {
		return nil, false
	}
	req = req.WithContext(e.t.Context())
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, true
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func sitesUpdate(spa *bool) sites.UpdateInput {
	return sites.UpdateInput{SpaMode: spa}
}

func newSiteRequest(t *testing.T, host, target, remote, forwarded string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+host+target, nil)
	req.Host = host
	req.RemoteAddr = remote
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	return req
}

func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func analyticsUpdate(enabled *bool) sites.UpdateInput {
	return sites.UpdateInput{Analytics: enabled}
}
