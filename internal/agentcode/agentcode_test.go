package agentcode

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const testPassword = "correct horse battery"

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
	box, err := secrets.New(bytes.Repeat([]byte{8}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	recorder := mail.NewRecorder()
	if mailer == nil {
		mailer = recorder
	}
	root, err := authSvc.Setup(ctx, "alex@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	sam, err := authSvc.CreateUser(ctx, "sam@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(db, authSvc, box, Options{
		BaseURL: "https://sites.example.com", Mailer: mailer, Now: clk.Now, Logger: logger, Spawn: func(task func()) { task() },
	})
	return &fixture{svc: svc, auth: authSvc, mailer: recorder, clock: clk, root: root, sam: sam}
}

var codePattern = regexp.MustCompile(`\n(\d{8})\n`)

func (f *fixture) lastCode(t *testing.T) string {
	t.Helper()
	sent := f.mailer.Messages()
	if len(sent) == 0 {
		t.Fatal("no email was sent")
	}
	match := codePattern.FindStringSubmatch(sent[len(sent)-1].Text)
	if match == nil {
		t.Fatalf("no code in %q", sent[len(sent)-1].Text)
	}
	return match[1]
}

func wrongCode(code string) string {
	if code == "00000000" {
		return "11111111"
	}
	return "00000000"
}

func TestRequestFailsClearlyWithoutEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, mail.Unconfigured{})
		if f.svc.Available() {
			t.Error("Available is true without a mail sender")
		}
		if err := f.svc.Request(context.Background(), "sam@example.com"); !errors.Is(err, ErrEmailNotConfigured) {
			t.Fatalf("error = %v, want ErrEmailNotConfigured", err)
		}
		if err := f.svc.Request(context.Background(), "nobody@example.com"); !errors.Is(err, ErrEmailNotConfigured) {
			t.Fatalf("error for an unknown address = %v, want ErrEmailNotConfigured", err)
		}
	})
}

func TestRequestAnswersTheSameForEveryAddress(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if _, err := f.auth.CreateUser(ctx, "kim@example.com", testPassword); err != nil {
			t.Fatal(err)
		}
		if _, err := f.auth.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		for _, email := range []string{"nobody@example.com", "sam@example.com", "not an email", ""} {
			if err := f.svc.Request(ctx, email); err != nil {
				t.Errorf("Request(%q) = %v, want nil", email, err)
			}
		}
		if len(f.mailer.Messages()) != 0 {
			t.Fatalf("mail went to an unknown or disabled address: %+v", f.mailer.Messages())
		}
		if err := f.svc.Request(ctx, "kim@example.com"); err != nil {
			t.Fatal(err)
		}
		sent := f.mailer.Messages()
		if len(sent) != 1 || sent[0].To != "kim@example.com" || !strings.Contains(sent[0].Text, "agent") || !strings.Contains(sent[0].Text, "10 minutes") {
			t.Fatalf("messages = %+v", sent)
		}
	})
}

func TestVerifyReturnsAWorkingKeyOnce(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "Sam@Example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		key, record, err := f.svc.Verify(ctx, " sam@example.com ", code, "build agent")
		if err != nil {
			t.Fatal(err)
		}
		if record.Label != "build agent" || record.UserID != f.sam.ID {
			t.Errorf("record = %+v", record)
		}
		user, _, err := f.auth.AuthenticateAPIKey(ctx, key)
		if err != nil || user.ID != f.sam.ID {
			t.Fatalf("the key does not work: %v", err)
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, ""); !errors.Is(err, ErrCodeInvalid) {
			t.Errorf("reusing the code got %v, want ErrCodeInvalid", err)
		}
	})
}

func TestVerifyDefaultsTheLabel(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		_, record, err := f.svc.Verify(ctx, "sam@example.com", f.lastCode(t), "  ")
		if err != nil || record.Label != "agent" {
			t.Fatalf("label = %q, %v; want agent", record.Label, err)
		}
	})
}

func TestVerifyKeepsTheCodeWhenTheLabelIsBad(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		var invalid *auth.ValidationError
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, strings.Repeat("x", 65)); !errors.As(err, &invalid) {
			t.Fatalf("error = %v, want a validation error", err)
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, "ok"); err != nil {
			t.Fatalf("the code was spent by a rejected label: %v", err)
		}
	})
}

func TestVerifyRejectsExpiredCodes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		f.clock.Advance(CodeLifetime + time.Second)
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, ""); !errors.Is(err, ErrCodeInvalid) {
			t.Fatalf("error = %v, want ErrCodeInvalid", err)
		}
	})
}

func TestVerifyLimitsAttempts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		for range CodeAttempts {
			if _, _, err := f.svc.Verify(ctx, "sam@example.com", wrongCode(code), ""); !errors.Is(err, ErrCodeInvalid) {
				t.Fatalf("wrong code error = %v", err)
			}
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, ""); !errors.Is(err, ErrCodeInvalid) {
			t.Fatalf("the right code worked after %d wrong attempts: %v", CodeAttempts, err)
		}
	})
}

func TestVerifyAcceptsTheRightCodeOnTheLastAttempt(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		for range CodeAttempts - 1 {
			if _, _, err := f.svc.Verify(ctx, "sam@example.com", wrongCode(code), ""); !errors.Is(err, ErrCodeInvalid) {
				t.Fatal(err)
			}
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, ""); err != nil {
			t.Fatalf("the fifth attempt was refused: %v", err)
		}
	})
}

func TestANewRequestReplacesTheOldCode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		first := f.lastCode(t)
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		second := f.lastCode(t)
		if first != second {
			if _, _, err := f.svc.Verify(ctx, "sam@example.com", first, ""); !errors.Is(err, ErrCodeInvalid) {
				t.Errorf("the replaced code still works: %v", err)
			}
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", second, ""); err != nil {
			t.Errorf("the new code does not work: %v", err)
		}
	})
}

func TestVerifyRefusesADisabledUserAndAMalformedInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		for _, bad := range []string{"", "1234", "abcdefgh", "123456789"} {
			if _, _, err := f.svc.Verify(ctx, "sam@example.com", bad, ""); !errors.Is(err, ErrCodeInvalid) {
				t.Errorf("Verify(%q) = %v, want ErrCodeInvalid", bad, err)
			}
		}
		if _, _, err := f.svc.Verify(ctx, "not an email", code, ""); !errors.Is(err, ErrCodeInvalid) {
			t.Errorf("malformed email error = %v", err)
		}
		if _, _, err := f.svc.Verify(ctx, "kim@example.com", code, ""); !errors.Is(err, ErrCodeInvalid) {
			t.Errorf("another address error = %v", err)
		}
		if _, err := f.auth.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", code, ""); !errors.Is(err, ErrCodeInvalid) {
			t.Errorf("disabled user error = %v, want ErrCodeInvalid", err)
		}
	})
}

func TestVerifyRespectsTheKeyLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		for range auth.MaxActiveAPIKeys {
			if _, _, err := f.auth.CreateAPIKey(ctx, f.sam.ID, "filler"); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.svc.Verify(ctx, "sam@example.com", f.lastCode(t), ""); !errors.Is(err, auth.ErrAPIKeyLimit) {
			t.Fatalf("error = %v, want ErrAPIKeyLimit", err)
		}
	})
}

func TestAFailedSendDiscardsTheCode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		f.mailer.FailWith(errors.New("smtp is down"))
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_codes").Scan(&count); err != nil || count != 0 {
			t.Fatalf("codes left after a failed send = %d, %v", count, err)
		}
	})
}

func TestStoredCodesAreHashed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		code := f.lastCode(t)
		var stored string
		if err := db.QueryRowContext(ctx, "SELECT code_hash FROM agent_codes").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stored, code) || len(stored) < 32 {
			t.Errorf("stored hash = %q", stored)
		}
	})
}

func TestPurgeRemovesExpiredCodes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		if err := f.svc.Request(ctx, "sam@example.com"); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(CodeLifetime + time.Second)
		if err := f.svc.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_codes").Scan(&count); err != nil || count != 0 {
			t.Fatalf("rows left = %d, %v", count, err)
		}
	})
}
