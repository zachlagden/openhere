package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestDisabledAccountsAreCutOffEverywhere(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		cookie := h.sessionFor("sam@example.com")
		expectStatus(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)
		sam.expect(http.StatusOK, "GET", "/api/v1/me", nil)

		if _, err := h.auth.SetDisabled(t.Context(), root.userID, sam.userID, true); err != nil {
			t.Fatal(err)
		}
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized, codeUnauthenticated)
		sam.expectError(http.StatusUnauthorized, codeUnauthenticated, "GET", "/api/v1/me", nil)
		expectError(t, h.do("POST", "/api/v1/auth/login", loginRequest{Email: "sam@example.com", Password: testPassword}), http.StatusForbidden, codeAccountDisabled)
		expectError(t, h.do("POST", "/api/v1/auth/login", loginRequest{Email: "sam@example.com", Password: "not the password"}), http.StatusUnauthorized, codeInvalidLogin)
	})
}
