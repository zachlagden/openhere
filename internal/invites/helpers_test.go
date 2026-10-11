package invites

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/mail"
	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/workspaces"
)

const testPassword = "correct horse battery"

var testParams = auth.Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8}

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

type fixture struct {
	t        *testing.T
	db       store.Store
	svc      *Service
	auth     *auth.Service
	ws       *workspaces.Service
	settings *settings.Service
	mailer   *mail.Recorder
	clock    *clock
	root     auth.User
}

func newFixture(t *testing.T, db store.Store, mailer mail.Sender) *fixture {
	t.Helper()
	ctx := context.Background()
	clk := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, err := auth.New(db, auth.Options{KeyPrefix: "sk_", Params: testParams, Now: clk.Now, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	wsSvc := workspaces.New(db, workspaces.Options{Now: clk.Now})
	settingsSvc := settings.New(db, settings.Options{Now: clk.Now})
	recorder := mail.NewRecorder()
	if mailer == nil {
		mailer = recorder
	}
	root, err := authSvc.Setup(ctx, "alex@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(db, authSvc, wsSvc, settingsSvc, Options{BaseURL: "https://sites.example.com", Mailer: mailer, Now: clk.Now, Logger: logger})
	return &fixture{t: t, db: db, svc: svc, auth: authSvc, ws: wsSvc, settings: settingsSvc, mailer: recorder, clock: clk, root: root}
}

func (f *fixture) user(email string) auth.User {
	f.t.Helper()
	user, err := f.auth.CreateUser(context.Background(), email, testPassword)
	if err != nil {
		f.t.Fatal(err)
	}
	return user
}

func (f *fixture) setPolicy(value string) {
	f.t.Helper()
	patch := map[string]json.RawMessage{settings.KeyWhoCanInvite: json.RawMessage(strconv.Quote(value))}
	if _, err := f.settings.Update(context.Background(), patch); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) workspace(owner auth.User, name, slug string) workspaces.Workspace {
	f.t.Helper()
	acc, err := f.ws.CreateShared(context.Background(), owner.ID, name, slug)
	if err != nil {
		f.t.Fatal(err)
	}
	return acc.Workspace
}

func tokenOf(t *testing.T, link string) string {
	t.Helper()
	token, found := strings.CutPrefix(link, "https://sites.example.com/invite/")
	if !found || len(token) != auth.TokenLength {
		t.Fatalf("link %q is not an invite link", link)
	}
	return token
}
