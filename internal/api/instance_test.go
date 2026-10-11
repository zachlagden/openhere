package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/docs"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestInstanceDescribesTheServerToSignedInUsers(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")

		got := decodeBody[instanceResponse](t, sam.expect(http.StatusOK, "GET", "/api/v1/instance", nil))
		want := instanceResponse{
			PublicURL: "http://example.com", Version: "1.2.3", EmailAvailable: true, CanCreateWorkspaces: true,
			HostingModes:         instanceModesResponse{Subdomain: false, Path: true, CustomDomains: true},
			MaxDomainsPerSite:    10,
			WhoCanInvite:         "root",
			AgentCredentialsDir:  ".selfhost",
			AgentCredentialsPath: "~/.selfhost/credentials",
		}
		if got != want {
			t.Errorf("instance = %+v, want %+v", got, want)
		}

		root.setSettings(`{"settings":{"workspaces.whoCanCreate":"root","hosting.customDomainsEnabled":false,"hosting.maxDomainsPerSite":3,"invites.whoCanInvite":"workspaceAdmins"}}`)
		restricted := decodeBody[instanceResponse](t, sam.expect(http.StatusOK, "GET", "/api/v1/instance", nil))
		if restricted.CanCreateWorkspaces || restricted.HostingModes.CustomDomains {
			t.Errorf("restricted = %+v", restricted)
		}
		if restricted.MaxDomainsPerSite != 3 || restricted.WhoCanInvite != "workspaceAdmins" {
			t.Errorf("policy fields = %d, %q", restricted.MaxDomainsPerSite, restricted.WhoCanInvite)
		}
		if !decodeBody[instanceResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/instance", nil)).CanCreateWorkspaces {
			t.Error("the root user must always be able to create workspaces")
		}
	})
}

func TestInstanceReportsMissingEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{noMailer: true})
		sam := h.bearerActor("sam@example.com")
		if decodeBody[instanceResponse](t, sam.expect(http.StatusOK, "GET", "/api/v1/instance", nil)).EmailAvailable {
			t.Error("emailAvailable is true without a sender")
		}
	})
}

func TestInstanceAndDocsNeedCredentials(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()
		for _, path := range []string{"/api/v1/instance", "/api/v1/docs/openapi.yaml", "/api/v1/docs/agent-guide.md"} {
			expectError(t, h.do("GET", path, nil), http.StatusUnauthorized, codeUnauthenticated)
		}
	})
}

func TestDocsServeTheEmbeddedFiles(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		sam := h.bearerActor("sam@example.com")

		spec := sam.expect(http.StatusOK, "GET", "/api/v1/docs/openapi.yaml", nil)
		if got := spec.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
			t.Errorf("openapi Content-Type = %q", got)
		}
		if spec.Body.String() != string(docs.OpenAPI) || !strings.HasPrefix(spec.Body.String(), "openapi: 3.1.0") {
			t.Error("the openapi body differs from the embedded file")
		}
		guide := sam.expect(http.StatusOK, "GET", "/api/v1/docs/agent-guide.md", nil)
		if got := guide.Header().Get("Content-Type"); got != "text/markdown; charset=utf-8" {
			t.Errorf("guide Content-Type = %q", got)
		}
		if guide.Body.String() != string(docs.AgentGuide) || !strings.Contains(guide.Body.String(), "~/.selfhost/credentials") {
			t.Error("the guide body differs from the embedded file")
		}
		if guide.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("nosniff missing on the guide")
		}
	})
}
