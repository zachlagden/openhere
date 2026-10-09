package sites

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/validate"
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

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type env struct {
	t     *testing.T
	db    store.Store
	ws    *workspaces.Service
	svc   *Service
	clock *testClock
	alex  string
	sam   string
	kim   string
	acme  workspaces.Workspace
}

func newEnv(t *testing.T, db store.Store, opts Options) *env {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	ws := workspaces.New(db, workspaces.Options{Now: clock.Now})
	opts.Now = clock.Now
	e := &env{t: t, db: db, ws: ws, svc: New(db, ws, opts), clock: clock}
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

func (e *env) folder(name string) Folder {
	e.t.Helper()
	f, err := e.svc.CreateFolder(context.Background(), e.alex, e.acme.Slug, name)
	if err != nil {
		e.t.Fatalf("CreateFolder(%s): %v", name, err)
	}
	return f
}

func (e *env) site(slug, folderID string) Site {
	e.t.Helper()
	e.clock.Advance(time.Second)
	site, err := e.svc.CreateSite(context.Background(), CreateInput{
		UserID: e.alex, Workspace: e.acme.Slug, Slug: slug, DisplayName: slug, FolderID: folderID,
	})
	if err != nil {
		e.t.Fatalf("CreateSite(%s): %v", slug, err)
	}
	return site
}

func (e *env) slugs(userID string, f Filter) []string {
	e.t.Helper()
	page, err := e.svc.ListSites(context.Background(), userID, f)
	if err != nil {
		e.t.Fatalf("ListSites(%+v): %v", f, err)
	}
	result := make([]string, 0, len(page.Sites))
	for _, site := range page.Sites {
		result = append(result, site.Slug)
	}
	return result
}

func isValidation(err error) bool {
	var invalid *validate.Error
	return errors.As(err, &invalid)
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, s := range got {
		seen[s]++
	}
	for _, s := range want {
		seen[s]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
