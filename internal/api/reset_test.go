package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const newPassword = "a different long password"

func (h *harness) requestReset(email string, opts ...requestOption) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/v1/auth/password-reset/request", resetRequest{Email: email}, opts...)
}

func (h *harness) confirmReset(token, password string) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/v1/auth/password-reset/confirm", resetConfirmRequest{Token: token, Password: password})
}

func resetTokenFrom(t *testing.T, text string) string {
	t.Helper()
	_, rest, found := strings.Cut(text, "http://"+testHost+"/reset-password/")
	if !found || len(rest) < 43 {
		t.Fatalf("no reset link in %q", text)
	}
	return rest[:43]
}

func TestResetRequestAnswersTheSameForEveryone(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		h.bearerActor("sam@example.com")
		off := h.bearerActor("off@example.com")
		if _, err := h.auth.SetDisabled(t.Context(), root.userID, off.userID, true); err != nil {
			t.Fatal(err)
		}
		known := h.requestReset("sam@example.com")
		expectStatus(t, known, http.StatusOK)
		for _, email := range []string{"nobody@example.com", "off@example.com", "not an email", ""} {
			rec := h.requestReset(email)
			expectStatus(t, rec, http.StatusOK)
			if rec.Body.String() != known.Body.String() {
				t.Errorf("response for %q differs: %q vs %q", email, rec.Body.String(), known.Body.String())
			}
		}
		sent := h.mailer.Messages()
		if len(sent) != 1 || sent[0].To != "sam@example.com" {
			t.Fatalf("messages = %+v", sent)
		}

		quiet := newHarness(t, db, harnessOptions{noMailer: true})
		quiet.bearerActor("kim@example.com")
		rec := quiet.requestReset("kim@example.com")
		expectStatus(t, rec, http.StatusOK)
		if rec.Body.String() != known.Body.String() {
			t.Errorf("response without email differs: %q", rec.Body.String())
		}
	})
}

func TestPasswordResetFlow(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{logToBuf: true})
		sam := h.bearerActor("sam@example.com")
		cookie := h.sessionFor("sam@example.com")
		expectStatus(t, h.requestReset("sam@example.com"), http.StatusOK)
		token := resetTokenFrom(t, h.mailer.Messages()[0].Text)

		expectStatus(t, h.confirmReset(token, newPassword), http.StatusNoContent)
		expectStatus(t, h.do("POST", "/api/v1/auth/login", loginRequest{Email: "sam@example.com", Password: newPassword}), http.StatusOK)
		expectError(t, h.do("POST", "/api/v1/auth/login", loginRequest{Email: "sam@example.com", Password: testPassword}), http.StatusUnauthorized, codeInvalidLogin)
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized, codeUnauthenticated)
		sam.expect(http.StatusOK, "GET", "/api/v1/me", nil)
		expectError(t, h.confirmReset(token, "yet another long password"), http.StatusBadRequest, codeInvalidResetToken)
		if strings.Contains(h.logs.String(), token) || strings.Contains(h.logs.String(), newPassword) {
			t.Error("a reset token or password reached the logs")
		}
	})
}

func TestResetTokenFailuresLookTheSame(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.bearerActor("sam@example.com")
		expectStatus(t, h.requestReset("sam@example.com"), http.StatusOK)
		expired := resetTokenFrom(t, h.mailer.Messages()[0].Text)
		h.clock.Advance(time.Hour + time.Second)
		expectStatus(t, h.requestReset("sam@example.com"), http.StatusOK)
		live := resetTokenFrom(t, h.mailer.Messages()[1].Text)

		var first string
		for name, token := range map[string]string{"expired": expired, "unknown": strings.Repeat("A", 43), "short": "abc", "empty": ""} {
			rec := h.confirmReset(token, newPassword)
			expectError(t, rec, http.StatusBadRequest, codeInvalidResetToken)
			if first == "" {
				first = rec.Body.String()
			}
			if rec.Body.String() != first {
				t.Errorf("%s: body %q differs from %q", name, rec.Body.String(), first)
			}
		}
		expectError(t, h.confirmReset(live, "short"), http.StatusBadRequest, codeValidationFailed)
		expectStatus(t, h.confirmReset(live, newPassword), http.StatusNoContent)
	})
}

func TestResetRequestsAreRateLimitedPerEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.ResetRequestEmail = Rule{Max: 2, Window: time.Hour}
		})})
		h.bearerActor("sam@example.com")
		for _, email := range []string{"sam@example.com", "nobody@example.com"} {
			expectStatus(t, h.requestReset(email), http.StatusOK)
			expectStatus(t, h.requestReset(email), http.StatusOK)
			expectError(t, h.requestReset(email), http.StatusTooManyRequests, codeRateLimited)
			expectError(t, h.requestReset(" "+strings.ToUpper(email)+" "), http.StatusTooManyRequests, codeRateLimited)
		}
		expectStatus(t, h.requestReset("other@example.com"), http.StatusOK)
		h.clock.Advance(time.Hour + time.Second)
		expectStatus(t, h.requestReset("sam@example.com"), http.StatusOK)
	})
}

func TestResetRequestsAndConfirmationsAreRateLimitedPerIP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.ResetRequestIP = Rule{Max: 2, Window: time.Hour}
			l.ResetConfirm = Rule{Max: 2, Window: time.Hour}
		})})
		expectStatus(t, h.requestReset("a@example.com"), http.StatusOK)
		expectStatus(t, h.requestReset("b@example.com"), http.StatusOK)
		expectError(t, h.requestReset("c@example.com"), http.StatusTooManyRequests, codeRateLimited)
		expectStatus(t, h.requestReset("c@example.com", fromAddr("203.0.113.5:1000")), http.StatusOK)

		for range 2 {
			expectError(t, h.confirmReset(strings.Repeat("A", 43), newPassword), http.StatusBadRequest, codeInvalidResetToken)
		}
		expectError(t, h.confirmReset(strings.Repeat("A", 43), newPassword), http.StatusTooManyRequests, codeRateLimited)
	})
}

func TestAdminCanGenerateAResetLinkWithoutEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{noMailer: true, logToBuf: true})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		path := "/api/v1/admin/users/" + sam.userID + "/password-reset"

		expectError(t, h.do("POST", path, nil), http.StatusUnauthorized, codeUnauthenticated)
		sam.expectError(http.StatusNotFound, codeNotFound, "POST", path, nil)
		root.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/admin/users/missing/password-reset", nil)

		issued := decodeBody[adminResetLinkResponse](t, root.expect(http.StatusCreated, "POST", path, nil))
		token := resetTokenFrom(t, issued.Link)
		if want := h.clock.Now().Add(time.Hour); !issued.ExpiresAt.Equal(want) {
			t.Errorf("expiresAt = %v, want %v", issued.ExpiresAt, want)
		}
		expectStatus(t, h.confirmReset(token, newPassword), http.StatusNoContent)
		expectStatus(t, h.do("POST", "/api/v1/auth/login", loginRequest{Email: "sam@example.com", Password: newPassword}), http.StatusOK)
		if strings.Contains(h.logs.String(), token) {
			t.Error("a reset token reached the logs")
		}

		if _, err := h.auth.SetDisabled(t.Context(), root.userID, sam.userID, true); err != nil {
			t.Fatal(err)
		}
		root.expectError(http.StatusConflict, codeUserDisabled, "POST", path, nil)
	})
}

func TestPasswordResetCanRevokeAPIKeys(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		sam := h.bearerActor("sam@example.com")
		kim := h.bearerActor("kim@example.com")
		expectStatus(t, h.requestReset("sam@example.com"), http.StatusOK)
		token := resetTokenFrom(t, h.mailer.Messages()[0].Text)

		rec := h.do("POST", "/api/v1/auth/password-reset/confirm", map[string]any{"token": token, "password": newPassword, "revokeApiKeys": true})
		expectStatus(t, rec, http.StatusNoContent)
		expectError(t, h.do("GET", "/api/v1/me", nil, withBearer(sam.key)), http.StatusUnauthorized, codeUnauthenticated)
		kim.expect(http.StatusOK, "GET", "/api/v1/me", nil)
	})
}
