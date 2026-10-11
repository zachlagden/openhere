package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

type keyList struct {
	Keys []apiKeyResponse `json:"keys"`
}

func TestAPIKeyLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie

		created := h.createKey(cookie, "ci runner")
		if !strings.HasPrefix(created.Key, "sk_") || created.APIKey.Label != "ci runner" || !strings.HasPrefix(created.Key, created.APIKey.Prefix) {
			t.Errorf("created = %+v", created)
		}

		rec := h.do("GET", "/api/v1/me/keys", nil, withCookie(cookie))
		expectStatus(t, rec, http.StatusOK)
		if strings.Contains(rec.Body.String(), created.Key) {
			t.Error("the key listing contains the full key")
		}
		list := decodeBody[keyList](t, rec)
		if len(list.Keys) != 1 || list.Keys[0].ID != created.APIKey.ID || list.Keys[0].RevokedAt != nil {
			t.Fatalf("list = %+v", list)
		}

		me := decodeBody[meBody](t, h.do("GET", "/api/v1/me", nil, withBearer(created.Key)))
		if me.AuthMethod != "apiKey" || me.User.Email != "alex@example.com" {
			t.Errorf("me via key = %+v", me)
		}
		list = decodeBody[keyList](t, h.do("GET", "/api/v1/me/keys", nil, withBearer(created.Key)))
		if list.Keys[0].LastUsedAt == nil {
			t.Error("lastUsedAt not set after the key was used")
		}

		expectStatus(t, h.do("DELETE", "/api/v1/me/keys/"+created.APIKey.ID, nil, browser(), withCookie(cookie)), http.StatusNoContent)
		expectError(t, h.do("GET", "/api/v1/me", nil, withBearer(created.Key)), http.StatusUnauthorized, codeUnauthenticated)
		list = decodeBody[keyList](t, h.do("GET", "/api/v1/me/keys", nil, withCookie(cookie)))
		if len(list.Keys) != 1 || list.Keys[0].RevokedAt == nil {
			t.Errorf("revoked key should stay listed with revokedAt: %+v", list)
		}
		expectError(t, h.do("DELETE", "/api/v1/me/keys/missing", nil, browser(), withCookie(cookie)), http.StatusNotFound, codeNotFound)
	})
}

func TestBadBearerDoesNotFallBackToCookie(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie), withBearer("sk_"+longString(43))), http.StatusUnauthorized, codeUnauthenticated)
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie), withHeader("Authorization", "Basic abc")), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestAPIKeyValidationAndLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie

		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "  "}, browser(), withCookie(cookie)), http.StatusBadRequest, codeValidationFailed)
		expectError(t, h.do("POST", "/api/v1/me/keys", `{"label":"x","admin":true}`, browser(), withCookie(cookie)), http.StatusBadRequest, codeInvalidJSON)

		for range auth.MaxActiveAPIKeys {
			h.createKey(cookie, "key")
		}
		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "extra"}, browser(), withCookie(cookie)), http.StatusConflict, codeKeyLimitReached)
	})
}

func TestKeysBelongToTheirOwner(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		created := h.createKey(cookie, "mine")

		if _, err := h.auth.CreateUser(t.Context(), "sam@example.com", testPassword); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		login := h.do("POST", "/api/v1/auth/login", loginRequest{Email: "sam@example.com", Password: testPassword})
		expectStatus(t, login, http.StatusOK)
		sam := sessionCookie(login)

		list := decodeBody[keyList](t, h.do("GET", "/api/v1/me/keys", nil, withCookie(sam)))
		if len(list.Keys) != 0 {
			t.Errorf("another user's keys are visible: %+v", list)
		}
		expectError(t, h.do("DELETE", "/api/v1/me/keys/"+created.APIKey.ID, nil, browser(), withCookie(sam)), http.StatusNotFound, codeNotFound)
		expectStatus(t, h.do("GET", "/api/v1/me", nil, withBearer(created.Key)), http.StatusOK)
	})
}
