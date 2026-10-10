package access

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

var fastParams = auth.Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type env struct {
	t      *testing.T
	box    *secrets.Box
	db     store.Store
	svc    *Service
	mailer *mail.Recorder
	clock  *clock
	ws     *workspaces.Service
	sites  *sites.Service
	acme   workspaces.Workspace
	alex   string
	sam    string
	kim    string
	site   sites.Site
	other  sites.Site
}

func newEnv(t *testing.T, db store.Store) *env {
	t.Helper()
	clk := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	box := testBox(t, 1)
	recorder := mail.NewRecorder()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ws := workspaces.New(db, workspaces.Options{Now: clk.Now})
	e := &env{
		t: t, db: db, box: box, mailer: recorder, clock: clk, ws: ws,
		sites: sites.New(db, ws, sites.Options{Now: clk.Now, Logger: logger}),
		svc: New(db, Options{
			Mailer: recorder, Params: fastParams, Now: clk.Now, Logger: logger, Spawn: func(task func()) { task() }, Secrets: box,
		}),
	}
	e.alex = e.user("alex@example.com")
	e.sam = e.user("sam@example.com")
	e.kim = e.user("kim@example.com")
	acc, err := ws.CreateShared(context.Background(), e.alex, "Acme Ltd", "acme")
	if err != nil {
		t.Fatalf("CreateShared: %v", err)
	}
	e.acme = acc.Workspace
	e.site = e.newSite("handbook")
	e.other = e.newSite("journal")
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

func (e *env) newSite(slug string) sites.Site {
	e.t.Helper()
	site, err := e.sites.CreateSite(context.Background(), sites.CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: slug})
	if err != nil {
		e.t.Fatalf("CreateSite(%s): %v", slug, err)
	}
	return site
}

func (e *env) grant(email, role, scopeType, scopeID string) {
	e.t.Helper()
	in := workspaces.AddMemberInput{Email: email, Role: role, ScopeType: scopeType, ScopeID: scopeID}
	if _, err := e.ws.AddMember(context.Background(), e.alex, e.acme.Slug, in); err != nil {
		e.t.Fatalf("grant %s: %v", email, err)
	}
}

func (e *env) update(siteID string, in ConfigUpdate) Config {
	e.t.Helper()
	cfg, err := e.svc.Update(context.Background(), siteID, in)
	if err != nil {
		e.t.Fatalf("Update: %v", err)
	}
	return cfg
}

func (e *env) state(siteID string) State {
	e.t.Helper()
	st, err := e.svc.State(context.Background(), siteID)
	if err != nil {
		e.t.Fatalf("State: %v", err)
	}
	return st
}

func (e *env) epoch(siteID string) int64 {
	e.t.Helper()
	return e.state(siteID).Epoch
}

func (e *env) cookie(siteID string, method Method, subject string) string {
	e.t.Helper()
	st := e.state(siteID)
	value, _, err := e.svc.IssueCookie(context.Background(), siteID, st.Epoch, method, subject)
	if err != nil {
		e.t.Fatalf("IssueCookie: %v", err)
	}
	return value
}

func (e *env) allowed(siteID string, cookies ...string) bool {
	e.t.Helper()
	decision, err := e.svc.Check(context.Background(), siteID, cookies)
	if err != nil {
		e.t.Fatalf("Check: %v", err)
	}
	return decision.Allowed
}

func strptr(s string) *string { return &s }

func listptr(values ...string) *[]string {
	if values == nil {
		values = []string{}
	}
	return &values
}

func setPassword(value string) PasswordChange {
	return PasswordChange{Set: true, Value: &value}
}

func clearPassword() PasswordChange {
	return PasswordChange{Set: true}
}

func isValidation(err error) bool {
	var invalid *validate.Error
	return errors.As(err, &invalid)
}

var codePattern = regexp.MustCompile(`\b[0-9]{8}\b`)

func lastCode(t *testing.T, e *env) string {
	t.Helper()
	messages := e.mailer.Messages()
	if len(messages) == 0 {
		t.Fatal("no message was recorded")
	}
	return codeIn(t, messages[len(messages)-1])
}

func codeIn(t *testing.T, msg mail.Message) string {
	t.Helper()
	code := codePattern.FindString(msg.Text)
	if code == "" {
		t.Fatalf("no code in %q", msg.Text)
	}
	return code
}

func testBox(t *testing.T, seed byte) *secrets.Box {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{seed}, secrets.KeyBytes))
	if err != nil {
		t.Fatalf("secrets.New: %v", err)
	}
	return box
}
