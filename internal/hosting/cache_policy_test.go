package hosting

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestIsContentHashed(t *testing.T) {
	hashed := []string{
		"_next/static/chunks/main-app-a1b2c3d4e5f6a7b8.js",
		"assets/index-Bv3HVboL.js",
		"assets/index-Bv3HVboL.css",
		"static/js/main.8a1b2c3d.js",
		"static/js/main.8a1b2c3d.chunk.js",
		"static/css/2.8a1b2c3d.chunk.css",
		"build/app_9f8e7d6c5b.min.js",
		"site/assets/vendor.1a2b3c4d5e6f.woff2",
		"assets/index-Bv3HVboL.js.map",
	}
	for _, p := range hashed {
		if !isContentHashed(p) {
			t.Errorf("%q must count as content hashed", p)
		}
	}
	plain := []string{
		"index.html",
		"app.js",
		"assets/app.css",
		"assets/logo.png",
		"assets/index-Bv-HVboL.js",
		"main.8a1b2c3d.js",
		"images/logo-8a1b2c3d.png",
		"abc12345def/assets/logo.png",
		"assets/12345678.js",
		"assets/abcdefgh.js",
		"assets/abc-12345678.js",
		"assets/photo-2024.jpg",
		"assets/team-photo-2024.jpg",
		"assets/short-a1b2c3.js",
		"assets/main-" + strings.Repeat("a1", 33) + ".js",
		"assets",
		"assets/",
	}
	for _, p := range plain {
		if isContentHashed(p) {
			t.Errorf("%q must not count as content hashed", p)
		}
	}
}

func TestEverySiteFileRevalidatesUnlessContentHashed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{
			"index.html":                 "<p>",
			"assets/index-Bv3HVboL.js":   "hashed",
			"assets/app.css":             "body{}",
			"static/js/main.8a1b2c3d.js": "webpack",
			"hash.8a1b2c3d.js":           "loose",
			"assets/page-8a1b2c3d.html":  "<p>",
			"video.mp4":                  "mp4",
			"_next/static/chunk-1.js":    "console.log(1)",
		}})
		e.enableSubdomains()

		cases := []struct{ target, cacheControl string }{
			{"/", CacheFile},
			{"/assets/index-Bv3HVboL.js", CacheImmutable},
			{"/static/js/main.8a1b2c3d.js", CacheImmutable},
			{"/assets/app.css", CacheFile},
			{"/hash.8a1b2c3d.js", CacheFile},
			{"/assets/page-8a1b2c3d.html", CacheFile},
			{"/video.mp4", CacheFile},
			{"/_next/static/chunk-1.js", CacheFile},
		}
		for _, tc := range cases {
			rec := e.site(tc.target)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status %d", tc.target, rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.cacheControl {
				t.Errorf("%s: Cache-Control = %q, want %q", tc.target, got, tc.cacheControl)
			}
			if strings.Contains(rec.Header().Get("Cache-Control"), "must-revalidate") {
				t.Errorf("%s: must-revalidate is back", tc.target)
			}
		}
		if got := e.get(managementHost, "/s/demo/assets/app.css").Header().Get("Cache-Control"); got != CacheFile {
			t.Errorf("path mode Cache-Control = %q", got)
		}
	})
}

func TestRepublishedAssetRevalidatesWithItsETag(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "<p>", "assets/app.css": "one"}})
		e.enableSubdomains()
		etag := e.site("/assets/app.css").Header().Get("ETag")

		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "<p>", "assets/app.css": "two"}})
		rec := e.site("/assets/app.css", "If-None-Match", etag)
		if rec.Code != http.StatusOK || rec.Body.String() != "two" {
			t.Errorf("a stale ETag = %d %q, want the new body", rec.Code, rec.Body.String())
		}
		if again := e.site("/assets/app.css", "If-None-Match", rec.Header().Get("ETag")); again.Code != http.StatusNotModified {
			t.Errorf("the current ETag = %d", again.Code)
		}
	})
}
