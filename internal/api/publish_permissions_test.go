package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestPublishRoutePermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		sam := h.bearerActor("sam@example.com")
		kim := h.bearerActor("kim@example.com")
		lee := h.bearerActor("lee@example.com")
		alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read"})
		alex.addMember("acme", addMemberRequest{Email: "kim@example.com", Role: "write"})

		files := siteFiles{"index.html": "x"}
		live := alex.publishLive("guarded", files)
		pending := alex.republish("guarded", live.ID, files)
		alex.uploadFiles(pending, files)
		versions := "/api/v1/publish/guarded/versions/"
		manifest := manifestOf(files)

		type route struct {
			name   string
			method string
			path   string
			body   any
			write  bool
		}
		routes := []route{
			{"republish", "PUT", "/api/v1/publish/guarded", republishRequest{Files: manifest}, true},
			{"upload", "PUT", pending.Uploads[0].URL, "x", true},
			{"finalize", "POST", versions + pending.VersionID + "/finalize", nil, true},
			{"restore", "POST", versions + live.ID + "/restore", nil, true},
			{"delete version", "DELETE", versions + "0123456789abcdef0123456789abcdef", nil, true},
			{"list versions", "GET", "/api/v1/publish/guarded/versions", nil, false},
			{"live manifest", "GET", "/api/v1/sites/guarded/files", nil, false},
			{"live file", "GET", "/api/v1/sites/guarded/files/index.html", nil, false},
		}
		for _, r := range routes {
			lee.expectError(http.StatusNotFound, codeNotFound, r.method, r.path, r.body)
			if r.write {
				sam.expectError(http.StatusForbidden, codeForbidden, r.method, r.path, r.body)
			} else {
				sam.expect(http.StatusOK, r.method, r.path, r.body)
			}
		}
		for _, r := range routes[5:] {
			kim.expect(http.StatusOK, r.method, r.path, r.body)
		}
		kim.expect(http.StatusOK, "PUT", pending.Uploads[0].URL, "x")
		kim.expect(http.StatusOK, "POST", versions+pending.VersionID+"/finalize", nil)
		kim.expect(http.StatusOK, "POST", versions+live.ID+"/restore", nil)
		kim.expectError(http.StatusNotFound, codeNotFound, "DELETE", versions+"0123456789abcdef0123456789abcdef", nil)
		kim.expect(http.StatusNoContent, "DELETE", versions+pending.VersionID, nil)

		for _, r := range routes {
			rec := h.do(r.method, r.path, r.body)
			expectError(t, rec, http.StatusUnauthorized, codeUnauthenticated)
		}
	})
}

func TestPublishCreationPermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")
		alex.createAcmeFolder("Private")
		sam := h.bearerActor("sam@example.com")
		kim := h.bearerActor("kim@example.com")
		lee := h.bearerActor("lee@example.com")
		alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read"})
		alex.addMember("acme", addMemberRequest{Email: "kim@example.com", Role: "write", ScopeType: "folder", ScopeID: reports.ID})
		body := func(folder string) publishRequest {
			return publishRequest{Workspace: "acme", Folder: folder, Files: manifestOf(siteFiles{"index.html": "x"})}
		}

		lee.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/publish", body(""))
		sam.expectError(http.StatusForbidden, codeForbidden, "POST", "/api/v1/publish", body(""))
		sam.expectError(http.StatusForbidden, codeForbidden, "POST", "/api/v1/publish", body("Reports"))
		kim.expectError(http.StatusForbidden, codeForbidden, "POST", "/api/v1/publish", body(""))
		kim.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/publish", body("Private"))
		created := decodeBody[publishResponse](t, kim.expect(http.StatusCreated, "POST", "/api/v1/publish", body("Reports")))
		site := decodeBody[siteEnvelope](t, kim.expect(http.StatusOK, "GET", "/api/v1/sites/"+created.Slug, nil)).Site
		if site.FolderID == nil || *site.FolderID != reports.ID || site.Role != "write" {
			t.Errorf("site = %+v", site)
		}
		lee.expect(http.StatusCreated, "POST", "/api/v1/publish", publishRequest{Files: manifestOf(siteFiles{"index.html": "x"})})
	})
}
