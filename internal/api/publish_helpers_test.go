package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type siteFiles map[string]string

func sumOf(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func manifestOf(files siteFiles) []fileSpecRequest {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	specs := make([]fileSpecRequest, 0, len(paths))
	for _, p := range paths {
		specs = append(specs, fileSpecRequest{Path: p, Size: int64(len(files[p])), SHA256: sumOf(files[p])})
	}
	return specs
}

func (a actor) publishNewSite(slug string, files siteFiles) publishResponse {
	a.h.t.Helper()
	a.h.clock.Advance(time.Second)
	rec := a.expect(http.StatusCreated, "POST", "/api/v1/publish", publishRequest{Workspace: "acme", Slug: slug, Files: manifestOf(files)})
	return decodeBody[publishResponse](a.h.t, rec)
}

func (a actor) put(url, body string) *httptest.ResponseRecorder {
	a.h.t.Helper()
	return a.h.do("PUT", url, body, withBearer(a.key))
}

func (a actor) uploadFiles(p publishResponse, files siteFiles) {
	a.h.t.Helper()
	for _, upload := range p.Uploads {
		rec := a.put(upload.URL, files[upload.Path])
		expectStatus(a.h.t, rec, http.StatusOK)
	}
}

func (a actor) finalize(slug, versionID string) *httptest.ResponseRecorder {
	a.h.t.Helper()
	return a.call("POST", "/api/v1/publish/"+slug+"/versions/"+versionID+"/finalize", nil)
}

func (a actor) publishLive(slug string, files siteFiles) versionResponse {
	a.h.t.Helper()
	p := a.publishNewSite(slug, files)
	a.uploadFiles(p, files)
	rec := a.finalize(p.Slug, p.VersionID)
	expectStatus(a.h.t, rec, http.StatusOK)
	return decodeBody[resultResponse](a.h.t, rec).Version
}

func (a actor) republish(slug, base string, files siteFiles) publishResponse {
	a.h.t.Helper()
	a.h.clock.Advance(time.Second)
	rec := a.expect(http.StatusCreated, "PUT", "/api/v1/publish/"+slug, republishRequest{BaseVersionID: base, Files: manifestOf(files)})
	return decodeBody[publishResponse](a.h.t, rec)
}

func (a actor) liveFile(slug, path string) *httptest.ResponseRecorder {
	a.h.t.Helper()
	return a.call("GET", "/api/v1/sites/"+slug+"/files/"+escapePath(path), nil)
}

func listTree(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		found = append(found, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(found)
	return found
}

func hasPrefixPath(paths []string, prefix string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
