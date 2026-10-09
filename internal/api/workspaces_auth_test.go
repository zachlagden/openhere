package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

type routeCase struct {
	method string
	path   string
	body   any
}

func workspaceRoutes() []routeCase {
	return []routeCase{
		{"GET", "/api/v1/workspaces", nil},
		{"POST", "/api/v1/workspaces", createWorkspaceRequest{Name: "Acme Ltd"}},
		{"GET", "/api/v1/workspaces/acme", nil},
		{"PATCH", "/api/v1/workspaces/acme", renameWorkspaceRequest{Name: "Acme Group"}},
		{"DELETE", "/api/v1/workspaces/acme", nil},
		{"GET", "/api/v1/workspaces/acme/members", nil},
		{"POST", "/api/v1/workspaces/acme/members", addMemberRequest{Email: "sam@example.com", Role: "read"}},
		{"PATCH", "/api/v1/workspaces/acme/members/abc", updateMemberRequest{Role: "read"}},
		{"DELETE", "/api/v1/workspaces/acme/members/abc", nil},
		{"GET", "/api/v1/workspaces/acme/folders", nil},
		{"POST", "/api/v1/workspaces/acme/folders", folderRequest{Name: "Reports"}},
		{"PATCH", "/api/v1/workspaces/acme/folders/abc", folderRequest{Name: "Reports"}},
		{"DELETE", "/api/v1/workspaces/acme/folders/abc", nil},
		{"POST", "/api/v1/workspaces/acme/sites", createSiteRequest{Slug: "launch"}},
		{"GET", "/api/v1/sites", nil},
		{"GET", "/api/v1/sites/launch", nil},
		{"PATCH", "/api/v1/sites/launch", `{"description":"x"}`},
		{"DELETE", "/api/v1/sites/launch", nil},
	}
}

func TestWorkspaceRoutesRequireAuthentication(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()
		for _, route := range workspaceRoutes() {
			expectError(t, h.do(route.method, route.path, route.body), http.StatusUnauthorized, codeUnauthenticated)
			expectError(t, h.do(route.method, route.path, route.body, withBearer("sk_"+longString(43))), http.StatusUnauthorized, codeUnauthenticated)
		}
	})
}

func TestWorkspaceRoutesEnforceCSRFForCookies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		for _, route := range workspaceRoutes() {
			if route.method == "GET" {
				continue
			}
			expectError(t, h.do(route.method, route.path, route.body, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
			expectError(t, h.do(route.method, route.path, route.body, withCookie(cookie), withHeader("X-Requested-With", "fetch")), http.StatusForbidden, codeCSRFRejected)
			expectError(t, h.do(route.method, route.path, route.body, withCookie(cookie), withHeader("Origin", "http://evil.example.net"), withHeader("X-Requested-With", "fetch")),
				http.StatusForbidden, codeCSRFRejected)
		}
	})
}

func TestWorkspaceFlowWithSessionCookie(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		rec := h.do("POST", "/api/v1/workspaces", createWorkspaceRequest{Name: "Acme Ltd"}, browser(), withCookie(cookie))
		expectStatus(t, rec, http.StatusCreated)
		expectStatus(t, h.do("GET", "/api/v1/workspaces/acme-ltd", nil, withCookie(cookie)), http.StatusOK)
		rec = h.do("POST", "/api/v1/workspaces/acme-ltd/sites", createSiteRequest{Slug: "launch"}, browser(), withCookie(cookie))
		expectStatus(t, rec, http.StatusCreated)
		expectStatus(t, h.do("DELETE", "/api/v1/sites/launch", nil, browser(), withCookie(cookie)), http.StatusNoContent)
	})
}

func TestWorkspaceBodiesNeedJSON(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		rec := h.do("POST", "/api/v1/workspaces", `{"name":"x"}`, withBearer(alex.key), withHeader("Content-Type", "text/plain"))
		expectError(t, rec, http.StatusUnsupportedMediaType, codeUnsupportedMedia)
	})
}
