package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type actor struct {
	h      *harness
	userID string
	key    string
}

func (h *harness) bearerActor(email string) actor {
	h.t.Helper()
	user, err := h.auth.CreateUser(context.Background(), email, testPassword)
	if err != nil {
		h.t.Fatalf("CreateUser(%s): %v", email, err)
	}
	key, _, err := h.auth.CreateAPIKey(context.Background(), user.ID, "tests")
	if err != nil {
		h.t.Fatalf("CreateAPIKey: %v", err)
	}
	return actor{h: h, userID: user.ID, key: key}
}

func (h *harness) rootActor() actor {
	h.t.Helper()
	setup := h.completeSetup()
	key := h.createKey(setup.cookie, "tests")
	return actor{h: h, userID: setup.user.ID, key: key.Key}
}

func (a actor) call(method, path string, body any) *httptest.ResponseRecorder {
	a.h.t.Helper()
	return a.h.do(method, path, body, withBearer(a.key))
}

func (a actor) expect(status int, method, path string, body any) *httptest.ResponseRecorder {
	a.h.t.Helper()
	rec := a.call(method, path, body)
	expectStatus(a.h.t, rec, status)
	return rec
}

func (a actor) expectError(status int, code, method, path string, body any) {
	a.h.t.Helper()
	expectError(a.h.t, a.call(method, path, body), status, code)
}

func (a actor) createWorkspace(name, slug string) workspaceResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusCreated, "POST", "/api/v1/workspaces", createWorkspaceRequest{Name: name, Slug: slug})
	return decodeBody[workspaceEnvelope](a.h.t, rec).Workspace
}

func (a actor) createAcmeFolder(name string) folderResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusCreated, "POST", "/api/v1/workspaces/acme/folders", folderRequest{Name: name})
	return decodeBody[folderEnvelope](a.h.t, rec).Folder
}

func (a actor) createSite(workspace string, req createSiteRequest) siteResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusCreated, "POST", "/api/v1/workspaces/"+workspace+"/sites", req)
	return decodeBody[siteEnvelope](a.h.t, rec).Site
}

func (a actor) addMember(workspace string, req addMemberRequest) memberResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusCreated, "POST", "/api/v1/workspaces/"+workspace+"/members", req)
	return decodeBody[memberEnvelope](a.h.t, rec).Member
}

func (a actor) siteSlugs(query string) []string {
	a.h.t.Helper()
	rec := a.expect(http.StatusOK, "GET", "/api/v1/sites"+query, nil)
	list := decodeBody[siteListResponse](a.h.t, rec)
	slugs := make([]string, 0, len(list.Sites))
	for _, site := range list.Sites {
		slugs = append(slugs, site.Slug)
	}
	return slugs
}

func sameSlugs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := map[string]int{}
	for _, s := range got {
		counts[s]++
	}
	for _, s := range want {
		counts[s]--
	}
	for _, n := range counts {
		if n != 0 {
			return false
		}
	}
	return true
}

func requireSlugs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !sameSlugs(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}
