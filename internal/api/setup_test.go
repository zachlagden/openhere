package api

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

type statusResponse struct {
	SetupRequired bool `json:"setupRequired"`
}

func TestSetupFlow(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		token := h.gate.Token()

		rec := h.do("GET", "/api/v1/setup/status", nil)
		expectStatus(t, rec, http.StatusOK)
		if !decodeBody[statusResponse](t, rec).SetupRequired {
			t.Error("setupRequired = false on an empty instance")
		}

		expectError(t, h.do("POST", "/api/v1/setup", setupRequest{Email: "alex@example.com", Password: testPassword, SetupToken: "wrong"}),
			http.StatusForbidden, codeInvalidSetupToken)
		expectError(t, h.do("POST", "/api/v1/setup", setupRequest{Email: "alex@example.com", Password: testPassword}),
			http.StatusForbidden, codeInvalidSetupToken)
		expectError(t, h.do("POST", "/api/v1/setup", setupRequest{Email: "alex@example.com", Password: "short", SetupToken: token}),
			http.StatusBadRequest, codeValidationFailed)
		expectStatus(t, h.do("GET", "/api/v1/setup/status", nil), http.StatusOK)

		rec = h.do("POST", "/api/v1/setup", setupRequest{Email: "Alex@Example.com", Password: testPassword, SetupToken: token})
		expectStatus(t, rec, http.StatusCreated)
		created := decodeBody[userEnvelope](t, rec).User
		if !created.IsRoot || created.Email != "alex@example.com" {
			t.Errorf("created user = %+v", created)
		}
		cookie := sessionCookie(rec)
		if cookie == nil {
			t.Fatal("no session cookie after setup")
		}
		expectStatus(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)

		rec = h.do("GET", "/api/v1/setup/status", nil)
		if decodeBody[statusResponse](t, rec).SetupRequired {
			t.Error("setupRequired = true after setup")
		}
		expectError(t, h.do("POST", "/api/v1/setup", setupRequest{Email: "sam@example.com", Password: testPassword, SetupToken: token}),
			http.StatusConflict, codeSetupComplete)
		if h.gate.Check(token) {
			t.Error("the setup token still works after setup")
		}
	})
}

func TestConcurrentSetupHasOneWinner(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		token := h.gate.Token()
		const attempts = 8
		statuses := make(chan int, attempts)
		var wg sync.WaitGroup
		for i := range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				email := fmt.Sprintf("user%d@example.com", i)
				rec := h.do("POST", "/api/v1/setup", setupRequest{Email: email, Password: testPassword, SetupToken: token})
				statuses <- rec.Code
			}()
		}
		wg.Wait()
		close(statuses)

		created, conflicts := 0, 0
		for status := range statuses {
			switch status {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				conflicts++
			default:
				t.Errorf("unexpected status %d", status)
			}
		}
		if created != 1 || conflicts != attempts-1 {
			t.Errorf("created = %d, conflicts = %d; want 1 and %d", created, conflicts, attempts-1)
		}
	})
}

func TestSetupRequiresJSONBody(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		token := h.gate.Token()
		valid := `{"email":"alex@example.com","password":"` + testPassword + `","setupToken":"` + token + `"}`

		rec := h.do("POST", "/api/v1/setup", valid, withHeader("Content-Type", "text/plain"))
		expectError(t, rec, http.StatusUnsupportedMediaType, codeUnsupportedMedia)
		rec = h.do("POST", "/api/v1/setup", `{"email":"alex@example.com","extra":1}`)
		expectError(t, rec, http.StatusBadRequest, codeInvalidJSON)
		rec = h.do("POST", "/api/v1/setup", `{"email":`)
		expectError(t, rec, http.StatusBadRequest, codeInvalidJSON)
		rec = h.do("POST", "/api/v1/setup", valid+`{}`)
		expectError(t, rec, http.StatusBadRequest, codeInvalidJSON)
		rec = h.do("POST", "/api/v1/setup", `{"email":"`+string(make([]byte, 0))+longString(maxBodyBytes+10)+`"}`)
		expectError(t, rec, http.StatusRequestEntityTooLarge, codeBodyTooLarge)
		rec = h.do("POST", "/api/v1/setup", nil)
		expectError(t, rec, http.StatusUnsupportedMediaType, codeUnsupportedMedia)
	})
}

func longString(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = 'a'
	}
	return string(buf)
}
