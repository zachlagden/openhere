package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/publish"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func (a actor) setSettings(body string) {
	a.h.t.Helper()
	a.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", body)
}

func TestPublishLimitsReturnStableCodes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		alex.setSettings(`{"settings":{"limits.maxFilesPerVersion":3,"limits.maxFileBytes":10,"limits.maxVersionBytes":25}}`)
		publishBody := func(files siteFiles) publishRequest {
			return publishRequest{Workspace: "acme", Files: manifestOf(files)}
		}
		alex.expectError(http.StatusRequestEntityTooLarge, codeTooManyFiles, "POST", "/api/v1/publish", publishBody(siteFiles{"a": "1", "b": "2", "c": "3", "d": "4"}))
		alex.expectError(http.StatusRequestEntityTooLarge, codeFileTooLarge, "POST", "/api/v1/publish", publishBody(siteFiles{"a": strings.Repeat("x", 11)}))
		alex.expectError(http.StatusRequestEntityTooLarge, codeVersionTooLarge, "POST", "/api/v1/publish",
			publishBody(siteFiles{"a": strings.Repeat("x", 10), "b": strings.Repeat("x", 10), "c": strings.Repeat("x", 6)}))
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/publish", publishRequest{Workspace: "acme"})
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/publish",
			publishRequest{Workspace: "acme", Files: []fileSpecRequest{{Path: "a", Size: 1, SHA256: "nope"}}})
		if page := alex.siteSlugs(""); len(page) != 0 {
			t.Errorf("rejected publishes created sites: %v", page)
		}
	})
}

func TestPublishRejectsUnsafePaths(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		bad := [][]string{
			{""}, {"/etc/passwd"}, {"a/./b"}, {"../x"}, {"a/../../x"}, {"a//b"}, {"dir/"}, {`a\b`}, {`..\..\x`}, {"a\u0000b"},
			{"a\nb"}, {strings.Repeat("a/", 600)}, {strings.Repeat("a", 256)}, {"a.html", "a.html"}, {"Readme.md", "README.MD"}, {"docs", "docs/x.html"},
		}
		for _, paths := range bad {
			files := make([]fileSpecRequest, len(paths))
			for i, p := range paths {
				files[i] = fileSpecRequest{Path: p, Size: 1, SHA256: sumOf("x")}
			}
			alex.expectError(http.StatusBadRequest, codeInvalidPath, "POST", "/api/v1/publish", publishRequest{Workspace: "acme", Files: files})
		}
		if page := alex.siteSlugs(""); len(page) != 0 {
			t.Errorf("rejected publishes created sites: %v", page)
		}
	})
}

func TestUploadRejectsTraversalAndWritesNothingOutside(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		p := alex.publishNewSite("traversal", siteFiles{"index.html": "x"})
		files := p.FinalizeURL[:len(p.FinalizeURL)-len("/finalize")] + "/files/"
		before := listTree(t, h.dataDir)

		for _, attempt := range []string{
			"../../../../../../tmp/evil", "..%2f..%2fevil", "%2e%2e/%2e%2e/evil", "a/../../evil", "./index.html", "index.html/", "a//b",
			"..%5c..%5cevil", "a%5cb", "%00", "index.html%00.png", "%2fetc%2fpasswd", "dir%2f..%2f..%2fevil",
		} {
			rec := alex.put(files+attempt, "x")
			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Errorf("PUT %s = %d, want 400 or 404; body: %s", attempt, rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("PUT %s Content-Type = %q", attempt, ct)
			}
		}
		after := listTree(t, h.dataDir)
		if strings.Join(before, "|") != strings.Join(after, "|") {
			t.Errorf("rejected uploads changed the data directory:\nbefore %v\nafter  %v", before, after)
		}
		expectStatus(t, alex.put(files+"index.html", "x"), http.StatusOK)
	})
}

func TestUploadErrors(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		p := alex.publishNewSite("upload-checks", siteFiles{"index.html": "hello world"})
		target := p.Uploads[0].URL
		base := strings.TrimSuffix(target, "index.html")

		expectError(t, alex.put(target, "jello world"), http.StatusBadRequest, codeHashMismatch)
		expectError(t, alex.put(target, "hello world!"), http.StatusBadRequest, codeSizeMismatch)
		expectError(t, alex.put(target, "hello"), http.StatusBadRequest, codeSizeMismatch)
		expectError(t, alex.put(target, ""), http.StatusBadRequest, codeSizeMismatch)
		expectError(t, alex.put(base+"other.html", "hello world"), http.StatusNotFound, codeFileNotInManifest)
		expectError(t, alex.put("/api/v1/publish/upload-checks/versions/0123456789abcdef0123456789abcdef/files/index.html", "x"), http.StatusNotFound, codeNotFound)
		expectError(t, alex.put("/api/v1/publish/ghost/versions/"+p.VersionID+"/files/index.html", "x"), http.StatusNotFound, codeNotFound)
		if tree := listTree(t, h.dataDir); hasPrefixPath(tree, "sites/"+p.SiteID) {
			t.Errorf("failed uploads left files: %v", tree)
		}

		expectStatus(t, alex.put(target, "hello world"), http.StatusOK)
		expectStatus(t, alex.finalize(p.Slug, p.VersionID), http.StatusOK)
		expectError(t, alex.put(target, "hello world"), http.StatusConflict, codeVersionFinalized)
	})
}

func TestUploadBodyIsCappedByTheFileLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		p := alex.publishNewSite("capped", siteFiles{"index.html": "tiny"})
		alex.setSettings(`{"settings":{"limits.maxFileBytes":4}}`)
		expectError(t, alex.put(p.Uploads[0].URL, strings.Repeat("x", 1<<20)), http.StatusRequestEntityTooLarge, codeFileTooLarge)
		alex.setSettings(`{"settings":{"limits.maxFileBytes":2}}`)
		expectError(t, alex.put(p.Uploads[0].URL, "tiny"), http.StatusRequestEntityTooLarge, codeFileTooLarge)
		if tree := listTree(t, h.dataDir); hasPrefixPath(tree, "sites/"+p.SiteID) {
			t.Errorf("rejected uploads left files: %v", tree)
		}
	})
}

func TestFinalizeReportsMissingFiles(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		files := siteFiles{"a.html": "aaa", "b.html": "bbb", "c/d.html": "ddd"}
		p := alex.publishNewSite("partial", files)
		expectStatus(t, alex.put(p.Uploads[0].URL, files[p.Uploads[0].Path]), http.StatusOK)

		rec := alex.finalize(p.Slug, p.VersionID)
		expectError(t, rec, http.StatusConflict, codeMissingFiles)
		body := decodeBody[errorBody](t, rec)
		if strings.Join(body.Missing, ",") != "b.html,c/d.html" {
			t.Errorf("missing = %v", body.Missing)
		}
		site := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/partial", nil)).Site
		if site.CurrentVersionID != nil {
			t.Error("an incomplete version went live")
		}
		for _, upload := range p.Uploads[1:] {
			expectStatus(t, alex.put(upload.URL, files[upload.Path]), http.StatusOK)
		}
		expectStatus(t, alex.finalize(p.Slug, p.VersionID), http.StatusOK)
	})
}

func TestExpiredPendingVersionsReturnGone(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		p := alex.publishNewSite("stale", siteFiles{"index.html": "x"})
		h.clock.Advance(publish.PendingTTL + time.Minute)
		expectError(t, alex.put(p.Uploads[0].URL, "x"), http.StatusGone, codeVersionExpired)
		expectError(t, alex.finalize(p.Slug, p.VersionID), http.StatusGone, codeVersionExpired)
		if err := h.publish.Maintain(t.Context()); err != nil {
			t.Fatal(err)
		}
		expectError(t, alex.finalize(p.Slug, p.VersionID), http.StatusNotFound, codeNotFound)
	})
}

func TestPublishRequestFormat(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		rec := h.do("POST", "/api/v1/publish", "files=1", withBearer(alex.key), withHeader("Content-Type", "text/plain"))
		expectError(t, rec, http.StatusUnsupportedMediaType, codeUnsupportedMedia)
		alex.expectError(http.StatusBadRequest, codeInvalidJSON, "POST", "/api/v1/publish", `{"files":[],"bogus":1}`)
		alex.expectError(http.StatusBadRequest, codeInvalidJSON, "POST", "/api/v1/publish", `{"files":[{"path":"a","size":"big","sha256":"x"}]}`)
		alex.expectError(http.StatusBadRequest, codeInvalidJSON, "PUT", "/api/v1/publish/anything", `{"workspace":"acme","files":[]}`)
		alex.expectError(http.StatusNotFound, codeNotFound, "PUT", "/api/v1/publish/ghost", republishRequest{Files: manifestOf(siteFiles{"a": "b"})})
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/publish/ghost/versions", nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/ghost/files", nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/ghost/files/index.html", nil)
	})
}

func TestPendingVersionLimitsReturnTooManyPending(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		live := alex.publishLive("busy", siteFiles{"index.html": "ok"})
		for range publish.MaxPendingPerSite {
			alex.expect(http.StatusCreated, "PUT", "/api/v1/publish/busy", republishRequest{BaseVersionID: live.ID, Files: manifestOf(siteFiles{"index.html": "next"})})
		}
		alex.expectError(http.StatusTooManyRequests, codeTooManyPending, "PUT", "/api/v1/publish/busy",
			republishRequest{BaseVersionID: live.ID, Files: manifestOf(siteFiles{"index.html": "next"})})

		alex.setSettings(`{"settings":{"limits.maxPendingBytes":4}}`)
		alex.expectError(http.StatusTooManyRequests, codeTooManyPending, "POST", "/api/v1/publish",
			publishRequest{Workspace: "acme", Files: manifestOf(siteFiles{"a": "12345"})})
	})
}
