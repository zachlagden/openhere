package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

var agentCodePattern = regexp.MustCompile(`\n(\d{8})\n`)

func (h *harness) requestAgentCode(email string, opts ...requestOption) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/v1/auth/agent/request-code", agentRequestCode{Email: email}, opts...)
}

func (h *harness) verifyAgentCode(email, code, label string) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/v1/auth/agent/verify-code", agentVerifyCode{Email: email, Code: code, Label: label})
}

func (h *harness) lastAgentCode() string {
	h.t.Helper()
	sent := h.mailer.Messages()
	if len(sent) == 0 {
		h.t.Fatal("no email was sent")
	}
	match := agentCodePattern.FindStringSubmatch(sent[len(sent)-1].Text)
	if match == nil {
		h.t.Fatalf("no code in %q", sent[len(sent)-1].Text)
	}
	return match[1]
}

func TestAgentRequestNeedsEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{noMailer: true})
		h.bearerActor("sam@example.com")
		for _, email := range []string{"sam@example.com", "nobody@example.com", ""} {
			body := expectError(t, h.requestAgentCode(email), http.StatusConflict, codeEmailNotConfigured)
			for _, want := range []string{"API key", "Settings", "dashboard"} {
				if !strings.Contains(body.Message, want) {
					t.Errorf("message %q does not mention %q", body.Message, want)
				}
			}
		}
	})
}

func TestAgentRequestAnswersTheSameForEveryAddress(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		h.bearerActor("sam@example.com")
		off := h.bearerActor("off@example.com")
		if _, err := h.auth.SetDisabled(t.Context(), root.userID, off.userID, true); err != nil {
			t.Fatal(err)
		}
		known := h.requestAgentCode("sam@example.com")
		expectStatus(t, known, http.StatusOK)
		for _, email := range []string{"nobody@example.com", "off@example.com", "not an email"} {
			rec := h.requestAgentCode(email)
			expectStatus(t, rec, http.StatusOK)
			if rec.Body.String() != known.Body.String() {
				t.Errorf("response for %q differs: %q vs %q", email, rec.Body.String(), known.Body.String())
			}
		}
		sent := h.mailer.Messages()
		if len(sent) != 1 || sent[0].To != "sam@example.com" || !strings.Contains(sent[0].Text, "agent") {
			t.Fatalf("messages = %+v", sent)
		}
	})
}

func TestAgentSignInGivesAKeyThatWorks(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{logToBuf: true})
		sam := h.bearerActor("sam@example.com")
		expectStatus(t, h.requestAgentCode("sam@example.com"), http.StatusOK)
		code := h.lastAgentCode()

		rec := h.verifyAgentCode("sam@example.com", code, "build agent")
		expectStatus(t, rec, http.StatusCreated)
		created := decodeBody[agentKeyResponse](t, rec)
		if !strings.HasPrefix(created.Key, "sk_") || created.APIKey.Label != "build agent" || created.CredentialsPath != "~/.selfhost/credentials" {
			t.Fatalf("created = %+v", created)
		}
		me := decodeBody[meResponse](t, h.do("GET", "/api/v1/me", nil, withBearer(created.Key)))
		if me.User.ID != sam.userID || me.AuthMethod != methodAPIKey {
			t.Errorf("me = %+v", me)
		}
		keys := decodeBody[keyListResponse](t, sam.expect(http.StatusOK, "GET", "/api/v1/me/keys", nil)).Keys
		found := false
		for _, key := range keys {
			found = found || key.Label == "build agent"
		}
		if !found {
			t.Errorf("the new key is not listed: %+v", keys)
		}
		expectError(t, h.verifyAgentCode("sam@example.com", code, ""), http.StatusBadRequest, codeInvalidCode)
		for _, secret := range []string{code, created.Key} {
			if strings.Contains(h.logs.String(), secret) {
				t.Error("a code or key reached the logs")
			}
		}
	})
}

func TestAgentKeyLabelDefaultsToAgent(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.bearerActor("sam@example.com")
		expectStatus(t, h.requestAgentCode("sam@example.com"), http.StatusOK)
		created := decodeBody[agentKeyResponse](t, h.verifyAgentCode("sam@example.com", h.lastAgentCode(), ""))
		if created.APIKey.Label != "agent" {
			t.Errorf("label = %q, want agent", created.APIKey.Label)
		}
	})
}

func TestAgentVerifyFailuresAreUniform(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		h.bearerActor("sam@example.com")
		off := h.bearerActor("off@example.com")
		expectStatus(t, h.requestAgentCode("sam@example.com"), http.StatusOK)
		code := h.lastAgentCode()
		wrong := "00000000"
		if code == wrong {
			wrong = "11111111"
		}
		var first string
		for name, rec := range map[string]*httptest.ResponseRecorder{
			"wrong code":      h.verifyAgentCode("sam@example.com", wrong, ""),
			"short code":      h.verifyAgentCode("sam@example.com", "123", ""),
			"letters":         h.verifyAgentCode("sam@example.com", "abcdefgh", ""),
			"unknown account": h.verifyAgentCode("nobody@example.com", code, ""),
			"another account": h.verifyAgentCode("off@example.com", code, ""),
			"bad email":       h.verifyAgentCode("not an email", code, ""),
		} {
			expectError(t, rec, http.StatusBadRequest, codeInvalidCode)
			if first == "" {
				first = rec.Body.String()
			}
			if rec.Body.String() != first {
				t.Errorf("%s: body %q differs from %q", name, rec.Body.String(), first)
			}
		}
		expectError(t, h.do("POST", "/api/v1/auth/agent/verify-code", `{"email":"sam@example.com","code":"`+code+`","extra":1}`), http.StatusBadRequest, codeInvalidJSON)
		expectError(t, h.verifyAgentCode("sam@example.com", code, strings.Repeat("x", 65)), http.StatusBadRequest, codeValidationFailed)

		if _, err := h.auth.SetDisabled(t.Context(), root.userID, off.userID, true); err != nil {
			t.Fatal(err)
		}
		if _, err := h.auth.SetDisabled(t.Context(), root.userID, h.userID("sam@example.com"), true); err != nil {
			t.Fatal(err)
		}
		expectError(t, h.verifyAgentCode("sam@example.com", code, ""), http.StatusBadRequest, codeInvalidCode)
	})
}

func (h *harness) userID(email string) string {
	h.t.Helper()
	user, err := h.auth.UserByEmail(h.t.Context(), email)
	if err != nil {
		h.t.Fatal(err)
	}
	return user.ID
}

func TestAgentCodeExpiresAfterTenMinutes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.bearerActor("sam@example.com")
		expectStatus(t, h.requestAgentCode("sam@example.com"), http.StatusOK)
		code := h.lastAgentCode()
		h.clock.Advance(10*time.Minute + time.Second)
		expectError(t, h.verifyAgentCode("sam@example.com", code, ""), http.StatusBadRequest, codeInvalidCode)
	})
}

func TestAgentCodeAllowsFiveAttempts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.bearerActor("sam@example.com")
		expectStatus(t, h.requestAgentCode("sam@example.com"), http.StatusOK)
		code := h.lastAgentCode()
		wrong := "00000000"
		if code == wrong {
			wrong = "11111111"
		}
		for range 5 {
			expectError(t, h.verifyAgentCode("sam@example.com", wrong, ""), http.StatusBadRequest, codeInvalidCode)
		}
		expectError(t, h.verifyAgentCode("sam@example.com", code, ""), http.StatusBadRequest, codeInvalidCode)
	})
}

func TestAgentSignInRespectsTheKeyLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		sam := h.bearerActor("sam@example.com")
		for range auth.MaxActiveAPIKeys - 1 {
			if _, _, err := h.auth.CreateAPIKey(t.Context(), sam.userID, "filler"); err != nil {
				t.Fatal(err)
			}
		}
		expectStatus(t, h.requestAgentCode("sam@example.com"), http.StatusOK)
		expectError(t, h.verifyAgentCode("sam@example.com", h.lastAgentCode(), ""), http.StatusConflict, codeKeyLimitReached)
	})
}

func TestAgentEndpointsAreRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		perEmail := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.AgentRequestEmail = Rule{Max: 3, Window: time.Hour}
		})})
		perEmail.bearerActor("sam@example.com")
		for _, email := range []string{"sam@example.com", "nobody@example.com"} {
			for range 3 {
				expectStatus(t, perEmail.requestAgentCode(email), http.StatusOK)
			}
			expectError(t, perEmail.requestAgentCode(email), http.StatusTooManyRequests, codeRateLimited)
		}
		perEmail.clock.Advance(time.Hour + time.Second)
		expectStatus(t, perEmail.requestAgentCode("sam@example.com"), http.StatusOK)
	})
}

func TestAgentRequestLimitsPerIPAndGlobally(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		byIP := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.AgentRequestIP = Rule{Max: 2, Window: time.Hour}
		})})
		expectStatus(t, byIP.requestAgentCode("a@example.com"), http.StatusOK)
		expectStatus(t, byIP.requestAgentCode("b@example.com"), http.StatusOK)
		expectError(t, byIP.requestAgentCode("c@example.com"), http.StatusTooManyRequests, codeRateLimited)
		expectStatus(t, byIP.requestAgentCode("c@example.com", fromAddr("203.0.113.5:1000")), http.StatusOK)

		global := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.AgentRequestAll = Rule{Max: 2, Window: time.Hour}
		})})
		expectStatus(t, global.requestAgentCode("a@example.com"), http.StatusOK)
		expectStatus(t, global.requestAgentCode("b@example.com", fromAddr("203.0.113.5:1000")), http.StatusOK)
		expectError(t, global.requestAgentCode("c@example.com", fromAddr("203.0.113.6:1000")), http.StatusTooManyRequests, codeRateLimited)
	})
}

func TestAgentVerifyIsRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.AgentVerifyEmail = Rule{Max: 2, Window: time.Hour}
			l.AgentVerifyIP = Rule{Max: 6, Window: time.Hour}
		})})
		for range 2 {
			expectError(t, h.verifyAgentCode("sam@example.com", "12345678", ""), http.StatusBadRequest, codeInvalidCode)
		}
		expectError(t, h.verifyAgentCode("sam@example.com", "12345678", ""), http.StatusTooManyRequests, codeRateLimited)
		for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
			expectError(t, h.verifyAgentCode(email, "12345678", ""), http.StatusBadRequest, codeInvalidCode)
		}
		expectError(t, h.verifyAgentCode("d@example.com", "12345678", ""), http.StatusTooManyRequests, codeRateLimited)
	})
}
