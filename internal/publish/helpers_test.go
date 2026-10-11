package publish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/sitefiles"
	"github.com/zachlagden/openhere/internal/sites"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
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

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type env struct {
	t        *testing.T
	db       store.Store
	dataDir  string
	clock    *testClock
	ws       *workspaces.Service
	sites    *sites.Service
	settings *settings.Service
	files    *sitefiles.Store
	svc      *Service
	alex     string
	sam      string
	kim      string
	acme     workspaces.Workspace
}

func newEnv(t *testing.T, db store.Store) *env {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dataDir := t.TempDir()
	files, err := sitefiles.New(dataDir)
	if err != nil {
		t.Fatalf("sitefiles.New: %v", err)
	}
	ws := workspaces.New(db, workspaces.Options{Now: clock.Now})
	siteSvc := sites.New(db, ws, sites.Options{Now: clock.Now, Files: files, Logger: logger})
	cfg := settings.New(db, settings.Options{Now: clock.Now})
	e := &env{
		t: t, db: db, dataDir: dataDir, clock: clock, ws: ws, sites: siteSvc, settings: cfg, files: files,
		svc: New(db, siteSvc, ws, cfg, files, Options{Now: clock.Now, Logger: logger}),
	}
	e.alex = e.user("alex@example.com")
	e.sam = e.user("sam@example.com")
	e.kim = e.user("kim@example.com")
	acc, err := ws.CreateShared(context.Background(), e.alex, "Acme Ltd", "acme")
	if err != nil {
		t.Fatalf("CreateShared: %v", err)
	}
	e.acme = acc.Workspace
	return e
}

func (e *env) user(email string) string {
	e.t.Helper()
	id := storetest.InsertUser(e.t, e.db, email)
	if _, err := workspaces.CreatePersonal(context.Background(), e.db, id, e.clock.Now()); err != nil {
		e.t.Fatalf("CreatePersonal: %v", err)
	}
	return id
}

func (e *env) grant(email, role, scopeType, scopeID string) {
	e.t.Helper()
	in := workspaces.AddMemberInput{Email: email, Role: role, ScopeType: scopeType, ScopeID: scopeID}
	if _, err := e.ws.AddMember(context.Background(), e.alex, e.acme.Slug, in); err != nil {
		e.t.Fatalf("grant %s %s on %s: %v", email, role, scopeType, err)
	}
}

func (e *env) set(raw string) {
	e.t.Helper()
	if _, err := e.settings.Update(context.Background(), parsePatch(e.t, raw)); err != nil {
		e.t.Fatalf("update settings %s: %v", raw, err)
	}
}

func hashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

type site map[string]string

func specs(files site) []FileSpec {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]FileSpec, 0, len(paths))
	for _, p := range paths {
		out = append(out, FileSpec{Path: p, Size: int64(len(files[p])), SHA256: hashOf(files[p])})
	}
	return out
}

func (e *env) upload(userID string, p Pending, files site) {
	e.t.Helper()
	for _, spec := range p.Files {
		body := files[spec.Path]
		if _, err := e.svc.Upload(context.Background(), userID, p.Site.Slug, p.Version.ID, spec.Path, strings.NewReader(body)); err != nil {
			e.t.Fatalf("Upload(%s): %v", spec.Path, err)
		}
	}
}

func (e *env) begin(slug string, files site) Pending {
	e.t.Helper()
	p, err := e.svc.Begin(context.Background(), BeginInput{
		UserID: e.alex, Workspace: e.acme.Slug, Slug: slug, Files: specs(files),
	})
	if err != nil {
		e.t.Fatalf("Begin(%s): %v", slug, err)
	}
	return p
}

func (e *env) publishNew(slug string, files site) Result {
	e.t.Helper()
	p := e.begin(slug, files)
	e.upload(e.alex, p, files)
	r, err := e.svc.Finalize(context.Background(), e.alex, slug, p.Version.ID)
	if err != nil {
		e.t.Fatalf("Finalize: %v", err)
	}
	return r
}

func (e *env) next(slug, base string, files site) Pending {
	e.t.Helper()
	e.clock.Advance(time.Second)
	p, err := e.svc.NewVersion(context.Background(), NewVersionInput{UserID: e.alex, Site: slug, BaseVersionID: base, Files: specs(files)})
	if err != nil {
		e.t.Fatalf("NewVersion(%s): %v", slug, err)
	}
	return p
}

func (e *env) republish(slug, base string, files site) Result {
	e.t.Helper()
	p := e.next(slug, base, files)
	e.upload(e.alex, p, files)
	r, err := e.svc.Finalize(context.Background(), e.alex, slug, p.Version.ID)
	if err != nil {
		e.t.Fatalf("Finalize: %v", err)
	}
	return r
}

func (e *env) versionDir(siteID, versionID string) string {
	return filepath.Join(e.dataDir, "sites", siteID, "versions", versionID)
}

func (e *env) siteDir(siteID string) string {
	return filepath.Join(e.dataDir, "sites", siteID)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (e *env) versionCount(siteID string) int {
	e.t.Helper()
	var n int
	if err := e.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM versions WHERE site_id = $1", siteID).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) readLive(slug, path string) string {
	e.t.Helper()
	live, err := e.svc.OpenLiveFile(context.Background(), e.alex, slug, path)
	if err != nil {
		e.t.Fatalf("OpenLiveFile(%s): %v", path, err)
	}
	body, err := io.ReadAll(live.File)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := live.File.Close(); err != nil {
		e.t.Fatal(err)
	}
	return string(body)
}

func isErr(err error, target error) bool { return errors.Is(err, target) }

func parsePatch(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad patch %s: %v", raw, err)
	}
	return out
}

func damage(t *testing.T, versionDir, rel, content string) {
	t.Helper()
	target := filepath.Join(versionDir, "files", filepath.FromSlash(rel))
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

var listAll = sites.Filter{}
