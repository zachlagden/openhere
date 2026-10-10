package passwordreset

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const (
	oldPassword = "correct horse battery"
	newPassword = "a different long password"
)

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
	svc    *Service
	auth   *auth.Service
	mailer *mail.Recorder
	clock  *clock
	root   auth.User
	sam    auth.User
}

func newFixture(t *testing.T, db store.Store, mailer mail.Sender) *fixture {
	t.Helper()
	ctx := context.Background()
	clk := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc, err := auth.New(db, auth.Options{
		KeyPrefix: "sk_", Params: auth.Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8}, Now: clk.Now, Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := mail.NewRecorder()
	if mailer == nil {
		mailer = recorder
	}
	root, err := authSvc.Setup(ctx, "alex@example.com", oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	sam, err := authSvc.CreateUser(ctx, "sam@example.com", oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(db, authSvc, Options{
		BaseURL: "https://sites.example.com", Mailer: mailer, Now: clk.Now, Logger: logger, Spawn: func(task func()) { task() },
	})
	return &fixture{svc: svc, auth: authSvc, mailer: recorder, clock: clk, root: root, sam: sam}
}

func tokenFrom(t *testing.T, text string) string {
	t.Helper()
	_, rest, found := strings.Cut(text, "https://sites.example.com/reset-password/")
	if !found || len(rest) < auth.TokenLength {
		t.Fatalf("no reset link in %q", text)
	}
	return rest[:auth.TokenLength]
}

func TestRequestEmailsALinkOnlyToEnabledExistingUsers(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		for _, email := range []string{"nobody@example.com", "not an email", ""} {
			if err := f.svc.Request(ctx, email); err != nil {
				t.Errorf("Request(%q) = %v, want nil", email, err)
			}
		}
		if _, err := f.auth.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		if len(f.mailer.Messages()) != 0 {
			t.Fatalf("mail went to an unknown or disabled address: %+v", f.mailer.Messages())
		}
		if _, err := f.auth.SetDisabled(ctx, f.root.ID, f.sam.ID, false); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Request(ctx, " SAM@example.com "); err != nil {
			t.Fatal(err)
		}
		sent := f.mailer.Messages()
		if len(sent) != 1 || sent[0].To != "sam@example.com" || !strings.Contains(sent[0].Text, "60 minutes") {
			t.Fatalf("messages = %+v", sent)
		}
	})
}

func TestRequestDoesNothingWithoutEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, mail.Unconfigured{})
		if err := f.svc.Request(context.Background(), "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM password_resets").Scan(&count); err != nil || count != 0 {
			t.Fatalf("reset rows = %d, %v; want none", count, err)
		}
	})
}

func TestConfirmCanAlsoRevokeAPIKeys(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		key, _, err := f.auth.CreateAPIKey(ctx, f.sam.ID, "laptop")
		if err != nil {
			t.Fatal(err)
		}
		other, _, err := f.auth.CreateAPIKey(ctx, f.root.ID, "roots laptop")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		token := tokenFrom(t, f.mailer.Messages()[0].Text)
		if err := f.svc.Confirm(ctx, token, newPassword, true); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.auth.AuthenticateAPIKey(ctx, key); !errors.Is(err, auth.ErrAPIKeyInvalid) {
			t.Errorf("the key survived a reset that asked to revoke keys: %v", err)
		}
		if _, _, err := f.auth.AuthenticateAPIKey(ctx, other); err != nil {
			t.Errorf("another user's key was revoked: %v", err)
		}
	})
}

func TestConfirmChangesThePasswordOnceAndEndsSessionsButKeepsKeys(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		session, _, err := f.auth.CreateSession(ctx, f.sam.ID)
		if err != nil {
			t.Fatal(err)
		}
		key, _, err := f.auth.CreateAPIKey(ctx, f.sam.ID, "laptop")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		token := tokenFrom(t, f.mailer.Messages()[0].Text)
		if err := f.svc.Confirm(ctx, token, newPassword, false); err != nil {
			t.Fatal(err)
		}
		if _, err := f.auth.Authenticate(ctx, "sam@example.com", newPassword); err != nil {
			t.Errorf("the new password does not work: %v", err)
		}
		if _, err := f.auth.Authenticate(ctx, "sam@example.com", oldPassword); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("the old password still works: %v", err)
		}
		if _, err := f.auth.ValidateSession(ctx, session); !errors.Is(err, auth.ErrSessionInvalid) {
			t.Errorf("the old session survived: %v", err)
		}
		if _, _, err := f.auth.AuthenticateAPIKey(ctx, key); err != nil {
			t.Errorf("the API key was revoked by a password reset: %v", err)
		}
		if err := f.svc.Confirm(ctx, token, "yet another long password", false); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("reusing the link got %v, want ErrInvalidToken", err)
		}
	})
}

func TestConfirmRejectsExpiredUnknownAndSupersededLinks(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		first := tokenFrom(t, f.mailer.Messages()[0].Text)
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		second := tokenFrom(t, f.mailer.Messages()[1].Text)
		if err := f.svc.Confirm(ctx, first, newPassword, false); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("a superseded link got %v, want ErrInvalidToken", err)
		}
		for _, token := range []string{"", "short", strings.Repeat("A", auth.TokenLength)} {
			if err := f.svc.Confirm(ctx, token, newPassword, false); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Confirm(%q) = %v, want ErrInvalidToken", token, err)
			}
		}
		f.clock.Advance(Lifetime + time.Second)
		if err := f.svc.Confirm(ctx, second, newPassword, false); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("an expired link got %v, want ErrInvalidToken", err)
		}
	})
}

func TestConfirmKeepsTheLinkWhenThePasswordIsRejected(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		token := tokenFrom(t, f.mailer.Messages()[0].Text)
		var invalid *auth.ValidationError
		if err := f.svc.Confirm(ctx, token, "short", false); !errors.As(err, &invalid) {
			t.Fatalf("short password error = %v", err)
		}
		if err := f.svc.Confirm(ctx, token, newPassword, false); err != nil {
			t.Fatalf("the link was spent by a rejected password: %v", err)
		}
	})
}

func TestConfirmFailsForADisabledUser(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		token := tokenFrom(t, f.mailer.Messages()[0].Text)
		if _, err := f.auth.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Confirm(ctx, token, newPassword, false); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, want ErrInvalidToken", err)
		}
	})
}

func TestConcurrentConfirmHasOneWinner(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		token := tokenFrom(t, f.mailer.Messages()[0].Text)
		results := make(chan error, 4)
		for range 4 {
			go func() { results <- f.svc.Confirm(ctx, token, newPassword, false) }()
		}
		wins := 0
		for range 4 {
			if err := <-results; err == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("%d confirmations succeeded, want exactly 1", wins)
		}
	})
}

func TestAdminLinkWorksWithoutEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, mail.Unconfigured{})
		ctx := context.Background()
		link, expires, err := f.svc.AdminLink(ctx, f.sam.ID)
		if err != nil {
			t.Fatal(err)
		}
		if expires.Sub(f.clock.Now()) != Lifetime {
			t.Errorf("expiry = %v", expires)
		}
		token := tokenFrom(t, link)
		if err := f.svc.Confirm(ctx, token, newPassword, false); err != nil {
			t.Fatal(err)
		}
		if _, err := f.auth.Authenticate(ctx, "sam@example.com", newPassword); err != nil {
			t.Errorf("the new password does not work: %v", err)
		}
		if _, _, err := f.svc.AdminLink(ctx, "no-such-user"); !errors.Is(err, auth.ErrUserNotFound) {
			t.Errorf("unknown user error = %v", err)
		}
		if _, err := f.auth.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.svc.AdminLink(ctx, f.sam.ID); !errors.Is(err, auth.ErrUserDisabled) {
			t.Errorf("disabled user error = %v", err)
		}
	})
}

func TestStoredTokensAreHashed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		link, _, err := f.svc.AdminLink(ctx, f.sam.ID)
		if err != nil {
			t.Fatal(err)
		}
		token := tokenFrom(t, link)
		var stored string
		if err := db.QueryRowContext(ctx, "SELECT token_hash FROM password_resets").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored == token || stored != auth.HashToken(token) {
			t.Errorf("stored hash = %q", stored)
		}
	})
}

func TestPurgeRemovesOldLinks(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if _, _, err := f.svc.AdminLink(ctx, f.sam.ID); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(Lifetime + 25*time.Hour)
		if err := f.svc.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM password_resets").Scan(&count); err != nil || count != 0 {
			t.Fatalf("rows left = %d, %v", count, err)
		}
	})
}
