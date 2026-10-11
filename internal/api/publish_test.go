package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

var sampleSite = siteFiles{
	"index.html":               "<h1>Hello</h1>",
	"assets/app.js":            "console.log('hi')",
	"my docs/ünï code.html":    "<p>spaced</p>",
	".well-known/security.txt": "Contact: mailto:alex@example.com",
}

func TestPublishFlowOverHTTP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")

		p := alex.publishNewSite("acme-docs", sampleSite)
		if p.Slug != "acme-docs" || len(p.VersionID) != 32 || len(p.Uploads) != len(sampleSite) || p.ExpiresAt.IsZero() {
			t.Fatalf("publish response = %+v", p)
		}
		prefix := "/api/v1/publish/acme-docs/versions/" + p.VersionID
		if p.FinalizeURL != prefix+"/finalize" {
			t.Errorf("finalizeUrl = %q", p.FinalizeURL)
		}
		for _, upload := range p.Uploads {
			if upload.Method != "PUT" || !strings.HasPrefix(upload.URL, prefix+"/files/") || upload.Headers["Content-Type"] == "" {
				t.Errorf("upload = %+v", upload)
			}
			if strings.ContainsAny(upload.URL, " ü") {
				t.Errorf("upload url is not escaped: %q", upload.URL)
			}
		}

		site := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs", nil)).Site
		if site.CurrentVersionID != nil || site.SpaMode {
			t.Errorf("site before finalize = %+v", site)
		}
		manifest := decodeBody[manifestResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs/files", nil))
		if manifest.VersionID != nil || len(manifest.Files) != 0 {
			t.Errorf("manifest before finalize = %+v", manifest)
		}

		alex.uploadFiles(p, sampleSite)
		finalized := alex.finalize(p.Slug, p.VersionID)
		expectStatus(t, finalized, http.StatusOK)
		result := decodeBody[resultResponse](t, finalized)
		if result.Slug != "acme-docs" || !result.Version.Live || result.Version.Status != "finalized" || result.Version.FileCount != 4 || result.Version.FinalizedAt == nil {
			t.Errorf("finalize result = %+v", result)
		}
		again := decodeBody[resultResponse](t, alex.finalize(p.Slug, p.VersionID))
		if again.Version.ID != result.Version.ID || !again.Version.Live {
			t.Errorf("second finalize = %+v", again)
		}

		site = decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs", nil)).Site
		if site.CurrentVersionID == nil || *site.CurrentVersionID != p.VersionID {
			t.Errorf("site after finalize = %+v", site)
		}
		manifest = decodeBody[manifestResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs/files", nil))
		if manifest.VersionID == nil || *manifest.VersionID != p.VersionID || len(manifest.Files) != 4 {
			t.Fatalf("manifest = %+v", manifest)
		}
		for _, f := range manifest.Files {
			if f.SHA256 != sumOf(sampleSite[f.Path]) || f.Size != int64(len(sampleSite[f.Path])) || f.ContentType == "" {
				t.Errorf("manifest entry = %+v", f)
			}
		}
		for path, want := range sampleSite {
			rec := alex.liveFile("acme-docs", path)
			expectStatus(t, rec, http.StatusOK)
			if rec.Body.String() != want {
				t.Errorf("%s = %q, want %q", path, rec.Body.String(), want)
			}
		}
	})
}

func TestLiveFileIsServedAsADownload(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		alex.publishLive("xss-check", siteFiles{"evil.html": "<script>alert(1)</script>"})

		rec := alex.liveFile("xss-check", "evil.html")
		expectStatus(t, rec, http.StatusOK)
		header := rec.Header()
		if header.Get("Content-Type") != "application/octet-stream" || header.Get("Content-Disposition") != "attachment" ||
			header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(header.Get("Content-Security-Policy"), "default-src 'none'") {
			t.Errorf("headers = %v", header)
		}
		if header.Get("ETag") != `"`+sumOf("<script>alert(1)</script>")+`"` {
			t.Errorf("ETag = %q", header.Get("ETag"))
		}
		if header.Get("Content-Length") != strconv.Itoa(len("<script>alert(1)</script>")) {
			t.Errorf("Content-Length = %q", header.Get("Content-Length"))
		}
		ranged := h.do("GET", "/api/v1/sites/xss-check/files/evil.html", nil, withBearer(alex.key), withHeader("Range", "bytes=0-7"))
		expectStatus(t, ranged, http.StatusPartialContent)
		if ranged.Body.String() != "<script>" {
			t.Errorf("range body = %q", ranged.Body.String())
		}
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/xss-check/files/missing.html", nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/xss-check/files/..%2f..%2fetc%2fpasswd", nil)
	})
}

func TestStaleBaseAndRestoreOverHTTP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")

		v1 := alex.publishLive("history", siteFiles{"index.html": "one"})
		p2 := alex.republish("history", v1.ID, siteFiles{"index.html": "two"})
		alex.uploadFiles(p2, siteFiles{"index.html": "two"})
		expectStatus(t, alex.finalize("history", p2.VersionID), http.StatusOK)

		rec := alex.call("PUT", "/api/v1/publish/history", republishRequest{BaseVersionID: v1.ID, Files: manifestOf(siteFiles{"index.html": "three"})})
		expectError(t, rec, http.StatusConflict, codeVersionConflict)
		conflict := decodeBody[errorBody](t, rec)
		if conflict.CurrentVersionID == nil || *conflict.CurrentVersionID != p2.VersionID {
			t.Errorf("conflict = %+v, want currentVersionId %s", conflict, p2.VersionID)
		}

		overwrite := alex.republish("history", "", siteFiles{"index.html": "three"})
		alex.uploadFiles(overwrite, siteFiles{"index.html": "three"})
		expectStatus(t, alex.finalize("history", overwrite.VersionID), http.StatusOK)

		restored := alex.expect(http.StatusOK, "POST", "/api/v1/publish/history/versions/"+v1.ID+"/restore", nil)
		if !decodeBody[resultResponse](t, restored).Version.Live {
			t.Error("restored version is not live")
		}
		if got := alex.liveFile("history", "index.html").Body.String(); got != "one" {
			t.Errorf("live content after restore = %q", got)
		}

		list := decodeBody[versionListResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/publish/history/versions", nil))
		if len(list.Versions) != 3 || list.Versions[2].ID != v1.ID || !list.Versions[2].Live || list.Versions[0].Live {
			t.Errorf("versions = %+v", list.Versions)
		}

		alex.expectError(http.StatusConflict, codeVersionLive, "DELETE", "/api/v1/publish/history/versions/"+v1.ID, nil)
		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/publish/history/versions/"+p2.VersionID, nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/publish/history/versions/"+p2.VersionID, nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/publish/history/versions/"+p2.VersionID+"/restore", nil)
	})
}

func TestRestoreOfAPendingVersionIsRefused(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		v1 := alex.publishLive("pending-restore", siteFiles{"index.html": "one"})
		p2 := alex.republish("pending-restore", v1.ID, siteFiles{"index.html": "two"})
		alex.expectError(http.StatusConflict, codeNotFinalized, "POST", "/api/v1/publish/pending-restore/versions/"+p2.VersionID+"/restore", nil)
	})
}

func TestDeletingASiteRemovesItsFilesOverHTTP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		alex.publishLive("delete-me", siteFiles{"index.html": "one"})
		alex.publishLive("keep-me", siteFiles{"index.html": "two"})
		site := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/delete-me", nil)).Site

		before := listTree(t, h.dataDir)
		if !hasPrefixPath(before, "sites/"+site.ID) {
			t.Fatalf("site directory missing: %v", before)
		}
		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/sites/delete-me", nil)
		after := listTree(t, h.dataDir)
		if hasPrefixPath(after, "sites/"+site.ID) {
			t.Errorf("site directory survived: %v", after)
		}
		keeper := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/keep-me", nil)).Site
		if !hasPrefixPath(after, "sites/"+keeper.ID) {
			t.Errorf("another site's directory was removed: %v", after)
		}
	})
}

func TestPublishPicksTheWorkspace(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		acme := alex.createWorkspace("Acme Ltd", "acme")
		files := manifestOf(siteFiles{"index.html": "x"})

		personal := decodeBody[publishResponse](t, alex.expect(http.StatusCreated, "POST", "/api/v1/publish", publishRequest{Files: files}))
		site := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/"+personal.Slug, nil)).Site
		if site.WorkspaceID != alex.userID {
			t.Errorf("default workspace = %s, want the personal workspace %s", site.WorkspaceID, alex.userID)
		}

		viaHeader := h.do("POST", "/api/v1/publish", publishRequest{Files: files}, withBearer(alex.key), withHeader("X-Workspace", "acme"))
		expectStatus(t, viaHeader, http.StatusCreated)
		site = decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/"+decodeBody[publishResponse](t, viaHeader).Slug, nil)).Site
		if site.WorkspaceID != acme.ID {
			t.Errorf("header workspace = %s, want %s", site.WorkspaceID, acme.ID)
		}

		bodyWins := h.do("POST", "/api/v1/publish", publishRequest{Workspace: acme.ID, Files: files}, withBearer(alex.key), withHeader("X-Workspace", "nowhere"))
		expectStatus(t, bodyWins, http.StatusCreated)
		expectError(t, h.do("POST", "/api/v1/publish", publishRequest{Files: files}, withBearer(alex.key), withHeader("X-Workspace", "nowhere")), http.StatusNotFound, codeNotFound)
	})
}

func TestPublishWithSpaModeAndPatch(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		alex.expect(http.StatusCreated, "POST", "/api/v1/publish", publishRequest{
			Workspace: "acme", Slug: "spa-app", SpaMode: true, DisplayName: "SPA", Description: "A single page app", Files: manifestOf(siteFiles{"index.html": "x"}),
		})
		site := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/spa-app", nil)).Site
		if !site.SpaMode || site.DisplayName != "SPA" || site.Description != "A single page app" {
			t.Errorf("site = %+v", site)
		}
		patched := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/spa-app", `{"spaMode":false}`)).Site
		if patched.SpaMode || patched.DisplayName != "SPA" {
			t.Errorf("patched = %+v", patched)
		}
		patched = decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/spa-app", `{"description":"Updated"}`)).Site
		if patched.SpaMode {
			t.Error("a patch without spaMode changed it")
		}
		patched = decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/spa-app", `{"spaMode":true}`)).Site
		if !patched.SpaMode {
			t.Error("spaMode did not switch on")
		}
		alex.expectError(http.StatusBadRequest, codeInvalidJSON, "PATCH", "/api/v1/sites/spa-app", `{"spaMode":"yes"}`)
	})
}

func TestBrowserSessionsFollowTheCSRFRules(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		body := publishRequest{Slug: "browser-site", Files: manifestOf(siteFiles{"index.html": "x"})}
		expectError(t, h.do("POST", "/api/v1/publish", body, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		rec := h.do("POST", "/api/v1/publish", body, withCookie(cookie), browser())
		expectStatus(t, rec, http.StatusCreated)
		p := decodeBody[publishResponse](t, rec)

		upload := p.Uploads[0].URL
		expectError(t, h.do("PUT", upload, "x", withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("PUT", upload, "x", withCookie(cookie), browser()), http.StatusOK)
		finalize := p.FinalizeURL
		expectError(t, h.do("POST", finalize, nil, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("POST", finalize, nil, withCookie(cookie), browser()), http.StatusOK)
		expectStatus(t, h.do("GET", "/api/v1/sites/browser-site/files", nil, withCookie(cookie)), http.StatusOK)
	})
}

func TestLargeManifestsFitTheirOwnBodyLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		files := siteFiles{}
		for i := range 600 {
			files["pages/page-"+strconv.Itoa(i)+".html"] = "page " + strconv.Itoa(i)
		}
		rec := alex.call("POST", "/api/v1/publish", publishRequest{Workspace: "acme", Slug: "big-site", Files: manifestOf(files)})
		expectStatus(t, rec, http.StatusCreated)
		if len(rec.Body.Bytes()) < 16<<10 {
			t.Fatalf("response is only %d bytes", rec.Body.Len())
		}
		alex.expectError(http.StatusRequestEntityTooLarge, codeBodyTooLarge, "POST", "/api/v1/workspaces", `{"name":"`+strings.Repeat("x", 20<<10)+`"}`)
	})
}
