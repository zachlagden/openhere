package hosting

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

var nextExport = siteSpec{files: map[string]string{
	"index.html":              "home",
	"about.html":              "about",
	"docs/index.html":         "docs index",
	"docs/intro.html":         "docs intro",
	"blog/index.html":         "blog index",
	"blog.html":               "blog page",
	"404.html":                "custom missing",
	"_next/static/chunk-1.js": "console.log(1)",
	"_next/static/style.css":  "body{}",
	"data.json":               `{"a":1}`,
	"file.txt":                "plain",
	"favicon.ico":             "icon",
	"unknown.weirdext":        "weird",
	"folder/deep/page.html":   "deep",
	"folder/deep/index.html":  "deep index",
	"no-extension":            "a file with no extension",
}}

func TestFileResolutionOrder(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		cases := []struct {
			target string
			status int
			body   string
		}{
			{"/", 200, "home"},
			{"/index.html", 200, "home"},
			{"/about", 200, "about"},
			{"/about.html", 200, "about"},
			{"/about/", 200, "about"},
			{"/docs/", 200, "docs index"},
			{"/docs/index.html", 200, "docs index"},
			{"/docs/intro", 200, "docs intro"},
			{"/blog", 200, "blog page"},
			{"/blog/", 200, "blog index"},
			{"/folder/deep/page", 200, "deep"},
			{"/folder/deep/", 200, "deep index"},
			{"/_next/static/chunk-1.js", 200, "console.log(1)"},
			{"/_next/static/style.css", 200, "body{}"},
			{"/no-extension", 200, "a file with no extension"},
			{"/data.json", 200, `{"a":1}`},
			{"/about.html.html", 404, "custom missing"},
			{"/nothing", 404, "custom missing"},
			{"/docs/missing", 404, "custom missing"},
			{"/_next/static/missing.js", 404, "custom missing"},
			{"/About", 404, "custom missing"},
			{"//about", 404, "custom missing"},
			{"/docs//intro", 404, "custom missing"},
		}
		for _, tc := range cases {
			rec := e.site(tc.target)
			if rec.Code != tc.status || rec.Body.String() != tc.body {
				t.Errorf("GET %s = %d %q, want %d %q", tc.target, rec.Code, rec.Body.String(), tc.status, tc.body)
			}
		}
	})
}

func TestDirectoryWithoutTrailingSlashRedirectsOnce(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		rec := e.site("/docs?x=1&y=2")
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/docs/?x=1&y=2" {
			t.Errorf("/docs = %d, Location %q", rec.Code, rec.Header().Get("Location"))
		}
		follow := e.site(rec.Header().Get("Location"))
		if follow.Code != 200 || follow.Body.String() != "docs index" {
			t.Errorf("following the redirect = %d %q", follow.Code, follow.Body.String())
		}

		path := e.get(managementHost, "/s/demo/docs")
		if path.Code != http.StatusMovedPermanently || path.Header().Get("Location") != "/s/demo/docs/" {
			t.Errorf("path mode /docs = %d, Location %q", path.Code, path.Header().Get("Location"))
		}
		bare := e.get(managementHost, "/s/demo?a=b")
		if bare.Code != http.StatusMovedPermanently || bare.Header().Get("Location") != "/s/demo/?a=b" {
			t.Errorf("bare slug = %d, Location %q", bare.Code, bare.Header().Get("Location"))
		}
		unknown := e.get(managementHost, "/s/nobody")
		if unknown.Code != http.StatusMovedPermanently || unknown.Header().Get("Location") != "/s/nobody/" {
			t.Errorf("an unknown slug must redirect like a known one: %d %q", unknown.Code, unknown.Header().Get("Location"))
		}
	})
}

func TestNoRedirectLoopsAnywhereInTheSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		targets := []string{"/", "/about", "/about/", "/docs", "/docs/", "/blog", "/blog/", "/folder/deep", "/folder/deep/", "/nothing", "/nothing/"}
		for _, target := range targets {
			current := target
			for hop := 0; hop < 3; hop++ {
				rec := e.site(current)
				if rec.Code != http.StatusMovedPermanently {
					break
				}
				current = rec.Header().Get("Location")
				if hop == 2 {
					t.Errorf("%s is still redirecting after three hops (%s)", target, current)
				}
			}
		}
	})
}

func TestSPAFallback(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{spa: true, files: map[string]string{
			"index.html": "shell", "app.js": "js", "404.html": "custom missing",
		}})
		e.enableSubdomains()

		for target, want := range map[string]string{
			"/dashboard": "shell", "/users/42/edit": "shell", "/users/": "shell", "/app.js": "js",
		} {
			expectBody(t, e.site(target), 200, want)
		}
		expectBody(t, e.site("/missing.js"), 404, "custom missing")
		expectBody(t, e.site("/assets/missing.png"), 404, "custom missing")
		expectBody(t, e.site("/v1.2/route"), 200, "shell")
	})
}

func TestSPAWithoutIndexFallsBackToNotFound(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{spa: true, files: map[string]string{"other.html": "x"}})
		e.enableSubdomains()
		expectBody(t, e.site("/anything"), 404, "Not found\n")
	})
}

func TestNonSPASitesNeverFallBackToIndex(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "shell"}})
		e.enableSubdomains()
		expectBody(t, e.site("/dashboard"), 404, "Not found\n")
	})
}

func TestCustomNotFoundPage(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		rec := e.site("/missing")
		if rec.Code != 404 || rec.Body.String() != "custom missing" {
			t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q", ct)
		}
		if rec.Header().Get("Content-Length") != "14" {
			t.Errorf("Content-Length = %q", rec.Header().Get("Content-Length"))
		}
		head := e.request("HEAD", siteHost, "/missing")
		if head.Code != 404 || head.Body.Len() != 0 {
			t.Errorf("HEAD = %d with %d body bytes", head.Code, head.Body.Len())
		}
	})
}

func TestContentTypesAndCaching(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{
			files: map[string]string{"index.html": "<p>", "app.js": "x", "logo.svg": "<svg/>", "stored.bin": "b", "mystery": "m", "font.woff2": "f"},
			types: map[string]string{"stored.bin": "application/x-custom"},
		})
		e.enableSubdomains()

		cases := []struct{ target, contentType, cacheControl string }{
			{"/", "text/html; charset=utf-8", "no-cache"},
			{"/stored.bin", "application/x-custom", CacheFile},
			{"/mystery", "application/octet-stream", CacheFile},
			{"/logo.svg", "image/svg+xml", CacheFile},
			{"/app.js", "text/javascript; charset=utf-8", CacheFile},
		}
		for _, tc := range cases {
			rec := e.site(tc.target)
			if got := rec.Header().Get("Content-Type"); got != tc.contentType {
				t.Errorf("%s: Content-Type = %q, want %q", tc.target, got, tc.contentType)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.cacheControl {
				t.Errorf("%s: Cache-Control = %q, want %q", tc.target, got, tc.cacheControl)
			}
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("%s: nosniff is missing", tc.target)
			}
			if rec.Header().Get("Content-Security-Policy") != "" {
				t.Errorf("%s: subdomain mode must not impose a CSP, got %q", tc.target, rec.Header().Get("Content-Security-Policy"))
			}
		}
	})
}

func TestStoredEmptyContentTypeUsesTheExtension(t *testing.T) {
	cases := map[string]string{
		"a.woff2": "font/woff2", "A.HTML": "text/html; charset=utf-8", "x.wasm": "application/wasm",
		"noext": "application/octet-stream", "weird.zzzzqq": "application/octet-stream",
	}
	for name, want := range cases {
		if got := contentTypeFor(fileRecord{Path: name}); got != want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", name, got, want)
		}
	}
	if got := contentTypeFor(fileRecord{Path: "a.html", ContentType: "text/plain"}); got != "text/plain" {
		t.Errorf("a stored type must win, got %q", got)
	}
}

func TestRangeAndConditionalRequests(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		big := strings.Repeat("0123456789", 1000)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "home", "big.bin": big}})
		e.enableSubdomains()

		full := e.site("/big.bin")
		etag := full.Header().Get("ETag")
		if etag != `"`+hashOf(big)+`"` || full.Header().Get("Accept-Ranges") != "bytes" || full.Header().Get("Last-Modified") == "" {
			t.Fatalf("headers: ETag %q, Accept-Ranges %q, Last-Modified %q", etag, full.Header().Get("Accept-Ranges"), full.Header().Get("Last-Modified"))
		}

		part := e.site("/big.bin", "Range", "bytes=10-19")
		if part.Code != http.StatusPartialContent || part.Body.String() != "0123456789" || part.Header().Get("Content-Range") != "bytes 10-19/10000" {
			t.Errorf("range = %d %q %q", part.Code, part.Body.String(), part.Header().Get("Content-Range"))
		}
		suffix := e.site("/big.bin", "Range", "bytes=-5")
		if suffix.Code != http.StatusPartialContent || suffix.Body.String() != "56789" {
			t.Errorf("suffix range = %d %q", suffix.Code, suffix.Body.String())
		}
		bad := e.site("/big.bin", "Range", "bytes=20000-")
		if bad.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Errorf("unsatisfiable range = %d", bad.Code)
		}
		ifRange := e.site("/big.bin", "Range", "bytes=0-3", "If-Range", `"other"`)
		if ifRange.Code != 200 || ifRange.Body.Len() != len(big) {
			t.Errorf("a stale If-Range must return the whole file, got %d with %d bytes", ifRange.Code, ifRange.Body.Len())
		}

		same := e.site("/big.bin", "If-None-Match", etag)
		if same.Code != http.StatusNotModified || same.Body.Len() != 0 {
			t.Errorf("If-None-Match = %d with %d bytes", same.Code, same.Body.Len())
		}
		other := e.site("/big.bin", "If-None-Match", `"different"`)
		if other.Code != 200 {
			t.Errorf("a different ETag = %d", other.Code)
		}
		since := e.site("/big.bin", "If-Modified-Since", full.Header().Get("Last-Modified"))
		if since.Code != http.StatusNotModified {
			t.Errorf("If-Modified-Since = %d", since.Code)
		}
		htmlHit := e.site("/", "If-None-Match", `"`+hashOf("home")+`"`)
		if htmlHit.Code != http.StatusNotModified || htmlHit.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("html revalidation = %d, Cache-Control %q", htmlHit.Code, htmlHit.Header().Get("Cache-Control"))
		}

		pathMode := e.get(managementHost, "/s/demo/big.bin", "Range", "bytes=0-9")
		if pathMode.Code != http.StatusPartialContent || pathMode.Body.String() != "0123456789" {
			t.Errorf("path mode range = %d %q", pathMode.Code, pathMode.Body.String())
		}
	})
}
