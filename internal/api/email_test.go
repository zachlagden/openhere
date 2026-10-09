package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/mail/smtptest"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const smtpPassword = "smtp-secret-example-value"

func (a actor) configureSMTP(server *smtptest.Server, username, password string) {
	a.h.t.Helper()
	body := map[string]any{
		"email.provider": "smtp", "email.fromAddress": "noreply@example.com", "email.fromName": "Acme Ltd",
		"email.smtp.host": server.Host(), "email.smtp.port": server.Port(), "email.smtp.security": "none",
		"email.smtp.username": username, "email.smtp.password": password,
	}
	encoded, err := json.Marshal(map[string]any{"settings": body})
	if err != nil {
		a.h.t.Fatal(err)
	}
	a.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", string(encoded))
}

func liveHarness(t *testing.T, db store.Store, server *smtptest.Server) *harness {
	t.Helper()
	opts := &liveMailOptions{timeout: 2 * time.Second}
	if server != nil {
		opts.tls = server.TLSConfig()
	}
	return newHarness(t, db, harnessOptions{liveMail: opts, logToBuf: true})
}

func TestEmailSettingsNeverExposeSecrets(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		server := smtptest.Start(t, smtptest.Options{Username: "alex", Password: smtpPassword})
		h := liveHarness(t, db, server)
		root := h.rootActor()
		root.configureSMTP(server, "alex", smtpPassword)
		root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"email.resend.apiKey":"re_example_key_value"}}`)

		rec := root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)
		for _, secret := range []string{smtpPassword, "re_example_key_value"} {
			if strings.Contains(rec.Body.String(), secret) {
				t.Fatalf("a secret appears in the settings response: %s", rec.Body.String())
			}
		}
		values := decodeBody[settingsEnvelope](t, rec).Settings
		if values["email.smtp.passwordSet"] != true || values["email.resend.apiKeySet"] != true || values["email.provider"] != "smtp" {
			t.Errorf("settings = %v", values)
		}
		if values["email.smtp.host"] != server.Host() || values["email.fromName"] != "Acme Ltd" {
			t.Errorf("non-secret settings are not readable: %v", values)
		}
		var stored string
		if err := db.QueryRowContext(t.Context(), "SELECT value FROM instance_settings WHERE key = 'email.smtp.password'").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stored, smtpPassword) {
			t.Error("the SMTP password is stored in plain text")
		}

		root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"email.smtp.password":""}}`)
		cleared := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		if cleared["email.smtp.passwordSet"] != false {
			t.Errorf("clearing the password was ignored: %v", cleared)
		}
		root.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/admin/settings", `{"settings":{"email.smtp.passwordSet":true}}`)
		if strings.Contains(h.logs.String(), smtpPassword) || strings.Contains(h.logs.String(), "re_example_key_value") {
			t.Errorf("a secret reached the logs: %s", h.logs.String())
		}
	})
}

func TestEmailTestSendsToTheRootUser(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		server := smtptest.Start(t, smtptest.Options{Username: "alex", Password: smtpPassword})
		h := liveHarness(t, db, server)
		root := h.rootActor()
		root.expectError(http.StatusConflict, codeEmailNotConfigured, "POST", "/api/v1/admin/email/test", nil)

		root.configureSMTP(server, "alex", smtpPassword)
		sent := decodeBody[emailTestResponse](t, root.expect(http.StatusOK, "POST", "/api/v1/admin/email/test", nil))
		if !sent.Sent || sent.To != "alex@example.com" {
			t.Fatalf("response = %+v", sent)
		}
		got := server.Messages()
		if len(got) != 1 || got[0].To[0] != "alex@example.com" || got[0].AuthUser != "alex" || !strings.Contains(got[0].Header("Subject"), "Test email") {
			t.Fatalf("messages = %+v", got)
		}
	})
}

func TestEmailTestIsRootOnlyAndRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		server := smtptest.Start(t, smtptest.Options{})
		h := newHarness(t, db, harnessOptions{
			liveMail: &liveMailOptions{timeout: 2 * time.Second},
			limits: &Limits{
				LoginPerIP: Rule{Max: 1000, Window: time.Hour}, LoginPerEmail: Rule{Max: 1000, Window: time.Hour},
				SetupPerIP: Rule{Max: 1000, Window: time.Hour}, SetupPerEmail: Rule{Max: 1000, Window: time.Hour},
				EmailTest: Rule{Max: 2, Window: time.Hour},
			},
		})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		root.configureSMTP(server, "", "")
		expectError(t, h.do("POST", "/api/v1/admin/email/test", nil), http.StatusUnauthorized, codeUnauthenticated)
		sam.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/admin/email/test", nil)
		root.expect(http.StatusOK, "POST", "/api/v1/admin/email/test", nil)
		root.expect(http.StatusOK, "POST", "/api/v1/admin/email/test", nil)
		root.expectError(http.StatusTooManyRequests, codeRateLimited, "POST", "/api/v1/admin/email/test", nil)
		if len(server.Messages()) != 2 {
			t.Errorf("messages = %d, want 2", len(server.Messages()))
		}
	})
}

func TestEmailTestErrorsAreSanitised(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		server := smtptest.Start(t, smtptest.Options{Username: "alex", Password: smtpPassword})
		h := liveHarness(t, db, server)
		root := h.rootActor()
		root.configureSMTP(server, "alex", "the-wrong-password-value")
		rec := root.call("POST", "/api/v1/admin/email/test", nil)
		body := expectError(t, rec, http.StatusBadGateway, codeEmailSendFailed)
		if !strings.Contains(body.Message, "username or password") {
			t.Errorf("message = %q", body.Message)
		}
		for _, leaked := range []string{"the-wrong-password-value", smtpPassword, "5.7.8", "smtptest"} {
			if strings.Contains(rec.Body.String(), leaked) || strings.Contains(h.logs.String(), leaked) {
				t.Errorf("%q leaked into a response or the logs", leaked)
			}
		}

		stalled := smtptest.Start(t, smtptest.Options{Stall: true})
		root.configureSMTP(stalled, "", "")
		timed := root.call("POST", "/api/v1/admin/email/test", nil)
		if body := expectError(t, timed, http.StatusBadGateway, codeEmailSendFailed); !strings.Contains(body.Message, "timed out") {
			t.Errorf("timeout message = %q", body.Message)
		}
	})
}

func TestEmailSettingsTakeEffectWithoutARestart(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		first := smtptest.Start(t, smtptest.Options{})
		second := smtptest.Start(t, smtptest.Options{})
		h := liveHarness(t, db, nil)
		root := h.rootActor()
		root.configureSMTP(first, "", "")
		root.expect(http.StatusOK, "POST", "/api/v1/admin/email/test", nil)
		root.configureSMTP(second, "", "")
		root.expect(http.StatusOK, "POST", "/api/v1/admin/email/test", nil)
		if len(first.Messages()) != 1 || len(second.Messages()) != 1 {
			t.Fatalf("messages = %d and %d, want 1 and 1", len(first.Messages()), len(second.Messages()))
		}
		root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"email.provider":"none"}}`)
		root.expectError(http.StatusConflict, codeEmailNotConfigured, "POST", "/api/v1/admin/email/test", nil)
		root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"email.provider":"smtp","email.smtp.host":""}}`)
		root.expectError(http.StatusConflict, codeEmailNotConfigured, "POST", "/api/v1/admin/email/test", nil)
	})
}

func TestEmailTestThroughResend(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		var authHeader string
		var calls int
		resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			calls++
			w.WriteHeader(http.StatusOK)
		}))
		defer resend.Close()
		h := newHarness(t, db, harnessOptions{liveMail: &liveMailOptions{resend: resend.URL, timeout: 2 * time.Second}})
		root := h.rootActor()
		root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings",
			`{"settings":{"email.provider":"resend","email.fromAddress":"noreply@example.com","email.resend.apiKey":"re_example_key_value"}}`)
		root.expect(http.StatusOK, "POST", "/api/v1/admin/email/test", nil)
		if calls != 1 || authHeader != "Bearer re_example_key_value" {
			t.Fatalf("calls = %d, auth = %q", calls, authHeader)
		}
	})
}

func TestViewerCodesFollowTheLiveEmailSettings(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		server := smtptest.Start(t, smtptest.Options{})
		h := newHarness(t, db, harnessOptions{liveMail: &liveMailOptions{timeout: 2 * time.Second}})
		setup := newAccessSetup(t, h)
		setup.alex.setAccess(map[string]any{"mode": "restricted", "emailAllowlist": []string{"viewer@example.com"}})
		if setup.alex.access().EmailCodeAvailable {
			t.Fatal("emailCodeAvailable is true before email is configured")
		}
		setup.alex.configureSMTP(server, "", "")
		if !setup.alex.access().EmailCodeAvailable {
			t.Fatal("emailCodeAvailable stayed false after email was configured")
		}
		if err := h.access.RequestCode(t.Context(), access.CodeRequest{SiteID: setup.siteID, SiteLabel: "Guarded", Email: "viewer@example.com"}); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for len(server.Messages()) == 0 && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		got := server.Messages()
		if len(got) != 1 || got[0].To[0] != "viewer@example.com" || !strings.Contains(got[0].Header("Subject"), "Guarded") {
			t.Fatalf("messages = %+v", got)
		}
		setup.alex.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"email.provider":"none"}}`)
		if setup.alex.access().EmailCodeAvailable {
			t.Error("emailCodeAvailable stayed true after email was switched off")
		}
	})
}
