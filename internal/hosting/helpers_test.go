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
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	managementHost = "sites.example.com"
	siteHost       = "demo.sites.example.com"
)

type env struct {
	t        *testing.T
	db       store.Store
	dataDir  string
	settings *settings.Service
	publish  *publish.Service
	sites    *sites.Service
	files    *sitefiles.Store
	resolver *Resolver
	probes   *Probes
	router   *Router
	alex     string
	managed  atomic.Int64
}

type siteSpec struct {
	files map[string]string
	types map[string]string
	spa   bool
}

func newEnv(t *testing.T, db store.Store) *env {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dataDir := t.TempDir()
	files, err := sitefiles.New(dataDir)
	if err != nil {
		t.Fatalf("sitefiles.New: %v", err)
	}
	e := &env{t: t, db: db, dataDir: dataDir, files: files}
	e.resolver = NewResolver(db, nil)
	e.settings = settings.New(db, settings.Options{DefaultBaseDomain: managementHost})
	ws := workspaces.New(db, workspaces.Options{})
	siteSvc := sites.New(db, ws, sites.Options{Files: files, Logger: logger, OnSiteChange: e.resolver.Invalidate})
	e.sites = siteSvc
	e.publish = publish.New(db, siteSvc, ws, e.settings, files, publish.Options{Logger: logger, OnSiteChange: e.resolver.Invalidate})
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
	public := config.PublicURL{Scheme: "https", Host: managementHost}
	e.router = New(management, e.settings, db, files, Options{Public: public, Probes: e.probes, Resolver: e.resolver, Logger: logger})
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
	ctx := context.Background()
	var pending publish.Pending
	var err error
	if _, lookupErr := e.publish.List(ctx, e.alex, slug); lookupErr != nil {
		pending, err = e.publish.Begin(ctx, publish.BeginInput{UserID: e.alex, Slug: slug, SpaMode: in.spa, Files: e.specs(in)})
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
