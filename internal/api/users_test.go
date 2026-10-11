package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestUserAdminIsRootOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		path := "/api/v1/admin/users/" + root.userID
		expectError(t, h.do("GET", "/api/v1/admin/users", nil), http.StatusUnauthorized, codeUnauthenticated)
		expectError(t, h.do("PATCH", path, `{"disabled":true}`), http.StatusUnauthorized, codeUnauthenticated)
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/admin/users", nil)
		sam.expectError(http.StatusNotFound, codeNotFound, "PATCH", path, `{"disabled":true}`)
		sam.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/admin/users/"+sam.userID, `{"disabled":true}`)
		got := decodeBody[adminUserListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/users", nil))
		if len(got.Users) != 2 {
			t.Fatalf("users = %+v", got.Users)
		}
	})
}

func TestDisablingAndEnablingAUser(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		cookie := h.sessionFor("sam@example.com")
		path := "/api/v1/admin/users/" + sam.userID

		off := decodeBody[adminUserEnvelope](t, root.expect(http.StatusOK, "PATCH", path, `{"disabled":true}`)).User
		if !off.Disabled || off.DisabledAt == nil || off.Email != "sam@example.com" || off.IsRoot {
			t.Errorf("user = %+v", off)
		}
		sam.expectError(http.StatusUnauthorized, codeUnauthenticated, "GET", "/api/v1/me", nil)
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized, codeUnauthenticated)
		listed := decodeBody[adminUserListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/users", nil))
		for _, user := range listed.Users {
			if user.ID == sam.userID && !user.Disabled {
				t.Error("the list does not show the user as disabled")
			}
		}

		on := decodeBody[adminUserEnvelope](t, root.expect(http.StatusOK, "PATCH", path, `{"disabled":false}`)).User
		if on.Disabled || on.DisabledAt != nil {
			t.Errorf("user = %+v", on)
		}
		sam.expect(http.StatusOK, "GET", "/api/v1/me", nil)
	})
}

func TestUserAdminGuardsAndValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.expectError(http.StatusConflict, codeCannotDisableSelf, "PATCH", "/api/v1/admin/users/"+root.userID, `{"disabled":true}`)
		root.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/admin/users/missing", `{"disabled":true}`)
		root.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/admin/users/"+root.userID, `{}`)
		root.expectError(http.StatusBadRequest, codeInvalidJSON, "PATCH", "/api/v1/admin/users/"+root.userID, `{"disabled":"yes"}`)
		root.expectError(http.StatusBadRequest, codeInvalidJSON, "PATCH", "/api/v1/admin/users/"+root.userID, `{"email":"x@example.com"}`)
		root.expect(http.StatusOK, "GET", "/api/v1/me", nil)
	})
}
