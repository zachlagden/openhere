package access

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func (e *env) restrictWith(emails, domains []string) {
	e.t.Helper()
	e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted"), Emails: listptr(emails...), Domains: listptr(domains...)})
}

func (e *env) request(email string) {
	e.t.Helper()
	if err := e.svc.RequestCode(context.Background(), CodeRequest{SiteID: e.site.ID, SiteLabel: "handbook", Email: email}); err != nil {
		e.t.Fatalf("RequestCode(%s): %v", email, err)
	}
}

func (e *env) verify(email, code string) bool {
	e.t.Helper()
	_, ok, err := e.svc.VerifyCode(context.Background(), e.site.ID, email, code)
	if err != nil {
		e.t.Fatalf("VerifyCode: %v", err)
	}
	return ok
}

func TestAllowlistedEmailGetsACode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("Sam@Example.com")

		messages := e.mailer.Messages()
		if len(messages) != 1 || messages[0].To != "sam@example.com" {
			t.Fatalf("messages = %+v", messages)
		}
		code := codeIn(t, messages[0])
		if len(code) != CodeDigits {
			t.Errorf("code = %q", code)
		}
		if !strings.Contains(messages[0].Subject, "handbook") || messages[0].Subject == "" {
			t.Errorf("subject = %q", messages[0].Subject)
		}
		if !e.verify("sam@example.com", code) {
			t.Error("the emailed code was refused")
		}
	})
}

func TestDomainAllowlistGetsACode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith(nil, []string{"example.org"})
		e.request("kim@example.org")
		e.request("kim@mail.example.org")
		e.request("kim@example.org.evil.test")
		e.request("kim@notexample.org")
		messages := e.mailer.Messages()
		if len(messages) != 1 || messages[0].To != "kim@example.org" {
			t.Fatalf("messages = %+v", messages)
		}
		if !e.verify("kim@example.org", codeIn(t, messages[0])) {
			t.Error("the domain code was refused")
		}
	})
}

func TestOtherAddressesGetNoMailAndNoError(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		for _, email := range []string{"kim@example.com", "", "not-an-email", strings.Repeat("a", 300) + "@example.com"} {
			e.request(email)
		}
		if got := e.mailer.Messages(); len(got) != 0 {
			t.Fatalf("mail went to addresses off the allowlist: %+v", got)
		}
		var rows int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM viewer_codes").Scan(&rows); err != nil || rows != 0 {
			t.Errorf("codes stored for outsiders: %d, err = %v", rows, err)
		}
	})
}

func TestNoCodeIsSentWithoutAMailer(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.svc = New(db, Options{Params: fastParams, Now: e.clock.Now})
		e.restrictWith([]string{"sam@example.com"}, nil)
		if e.svc.EmailCodeAvailable() {
			t.Fatal("an unconfigured mailer reported itself as available")
		}
		e.request("sam@example.com")
		var rows int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM viewer_codes").Scan(&rows); err != nil || rows != 0 {
			t.Errorf("codes stored without a mailer: %d, err = %v", rows, err)
		}
	})
}

func TestCodeExpiresAfterTenMinutes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		e.clock.Advance(CodeLifetime)
		if e.verify("sam@example.com", code) {
			t.Error("an expired code was accepted")
		}
		e.request("sam@example.com")
		e.clock.Advance(CodeLifetime - time.Second)
		if !e.verify("sam@example.com", lastCode(t, e)) {
			t.Error("a code was refused a second before it expired")
		}
	})
}

func TestCodeIsSingleUse(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		if !e.verify("sam@example.com", code) {
			t.Fatal("first use refused")
		}
		if e.verify("sam@example.com", code) {
			t.Error("a code worked twice")
		}
	})
}

func TestCodeAllowsFiveAttempts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		wrong := "00000000"
		if code == wrong {
			wrong = "11111111"
		}
		for i := 0; i < CodeAttempts-1; i++ {
			if e.verify("sam@example.com", wrong) {
				t.Fatal("a wrong code was accepted")
			}
		}
		if !e.verify("sam@example.com", code) {
			t.Error("the right code was refused on the last allowed attempt")
		}
	})
}

func TestCodeDiesAfterTooManyWrongAttempts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		wrong := "00000000"
		if code == wrong {
			wrong = "11111111"
		}
		for i := 0; i < CodeAttempts; i++ {
			e.verify("sam@example.com", wrong)
		}
		if e.verify("sam@example.com", code) {
			t.Error("the right code worked after five wrong attempts")
		}
		var rows int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM viewer_codes").Scan(&rows); err != nil || rows != 0 {
			t.Errorf("a spent code row is still stored: %d, err = %v", rows, err)
		}
	})
}

func TestANewCodeReplacesTheOldOne(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		first := lastCode(t, e)
		e.request("sam@example.com")
		second := lastCode(t, e)
		if first != second && e.verify("sam@example.com", first) {
			t.Error("the replaced code still works")
		}
		if !e.verify("sam@example.com", second) {
			t.Error("the newest code was refused")
		}
	})
}

func TestCodeWorksOnlyForItsEmailAndSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com", "kim@example.com"}, nil)
		e.update(e.other.ID, ConfigUpdate{Mode: strptr("restricted"), Emails: listptr("sam@example.com")})
		e.request("sam@example.com")
		code := lastCode(t, e)
		if e.verify("kim@example.com", code) {
			t.Error("a code worked for another address")
		}
		if _, ok, err := e.svc.VerifyCode(context.Background(), e.other.ID, "sam@example.com", code); err != nil || ok {
			t.Errorf("a code worked on another site: ok = %v, err = %v", ok, err)
		}
		if !e.verify("sam@example.com", code) {
			t.Error("the code stopped working for its own address")
		}
	})
}

func TestCodesNeedRestrictedMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("public")})
		if e.verify("sam@example.com", code) {
			t.Error("a code worked on a public site")
		}
	})
}

func TestMalformedCodesAreRefused(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		for _, code := range []string{"", "1234567", "123456789", "1234 5678", "abcdefgh", "１２３４５６７８", "-1234567"} {
			if e.verify("sam@example.com", code) {
				t.Errorf("code %q accepted", code)
			}
		}
		if e.verify("not-an-email", "12345678") {
			t.Error("an invalid email was accepted")
		}
	})
}

func TestStoredCodeIsNotThePlainCode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		var stored string
		if err := db.QueryRowContext(context.Background(), "SELECT code_hash FROM viewer_codes").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored == code || strings.Contains(stored, code) || len(stored) != 64 {
			t.Errorf("stored code hash = %q", stored)
		}
	})
}

func TestFailedDeliveryDiscardsTheCode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.mailer.FailWith(errors.New("smtp down"))
		e.request("sam@example.com")
		var rows int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM viewer_codes").Scan(&rows); err != nil || rows != 0 {
			t.Errorf("an undelivered code is still stored: %d, err = %v", rows, err)
		}
	})
}

func TestCodeIsRemovedWhenTheAddressLeavesTheAllowlist(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		code := lastCode(t, e)
		e.update(e.site.ID, ConfigUpdate{Emails: listptr("kim@example.com")})
		if e.verify("sam@example.com", code) {
			t.Error("a code worked after its address left the allowlist")
		}
	})
}

func TestPurgeRemovesExpiredCodesAndHandoffs(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.restrictWith([]string{"sam@example.com"}, nil)
		e.request("sam@example.com")
		if _, err := e.svc.IssueHandoff(ctx, e.site.ID, e.alex); err != nil {
			t.Fatal(err)
		}
		if err := e.svc.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if countRows(t, db, "viewer_codes") != 1 || countRows(t, db, "viewer_handoffs") != 1 {
			t.Fatal("Purge removed live rows")
		}
		e.clock.Advance(CodeLifetime)
		if err := e.svc.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if countRows(t, db, "viewer_codes") != 0 || countRows(t, db, "viewer_handoffs") != 0 {
			t.Error("Purge kept expired rows")
		}
	})
}

func countRows(t *testing.T, db store.Store, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
