package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const shellHTML = "<!doctype html><title>app</title><script type=\"module\" src=\"/assets/app-1.js\"></script>"

func builtFiles() fstest.MapFS {
	return fstest.MapFS{
		"index.html":          {Data: []byte(shellHTML)},
		"assets/app-1.js":     {Data: []byte("export {}")},
		"assets/app-1.css":    {Data: []byte("body{}")},
		"assets/font-1.woff2": {Data: []byte("wOF2")},
		"theme-init.js":       {Data: []byte("document.documentElement")},
		".gitkeep":            {Data: nil},
	}
}

var apiAnswer = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTeapot)
	_, _ = w.Write([]byte("{\"from\":\"api\"}"))
})

func wrapped(t *testing.T, files fstest.MapFS) http.Handler {
	t.Helper()
	shell, err := New(files)
	if err != nil {
		t.Fatal(err)
	}
	return shell.Wrap(apiAnswer)
}

func get(h http.Handler, method, target string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://example.com"+target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestShellRoutesServeTheIndexWithStrictHeaders(t *testing.T) {
	h := wrapped(t, builtFiles())
	for _, target := range []string{"/", "/login", "/setup", "/invite/abc", "/reset-password/abc", "/workspaces/acme", "/docs/", "/index.html", "/a/b/c"} {
		rec := get(h, "GET", target)
		if rec.Code != http.StatusOK || rec.Body.String() != shellHTML {
			t.Errorf("GET %s = %d %q", target, rec.Code, rec.Body.String())
		}
		for header, want := range map[string]string{
			"Cache-Control":          "no-cache",
			"Content-Type":           "text/html; charset=utf-8",
			"X-Content-Type-Options": "nosniff",
			"Referrer-Policy":        "no-referrer",
			"X-Frame-Options":        "DENY",
		} {
			if got := rec.Header().Get(header); got != want {
				t.Errorf("GET %s: %s = %q, want %q", target, header, got, want)
			}
		}
		csp := rec.Header().Get("Content-Security-Policy")
		for _, directive := range []string{"default-src 'self'", "script-src 'self'", "connect-src 'self'", "frame-ancestors 'none'", "img-src 'self' data:"} {
			if !strings.Contains(csp, directive) {
				t.Errorf("GET %s: CSP %q lacks %q", target, csp, directive)
			}
		}
		if strings.Contains(csp, "unsafe") {
			t.Errorf("CSP %q allows unsafe sources", csp)
		}
	}
}

func TestHashedAssetsAreImmutableAndOthersRevalidate(t *testing.T) {
	h := wrapped(t, builtFiles())
	cases := map[string]struct{ cache, contentType string }{
		"/assets/app-1.js":     {"public, max-age=31536000, immutable", "text/javascript"},
		"/assets/app-1.css":    {"public, max-age=31536000, immutable", "text/css"},
		"/assets/font-1.woff2": {"public, max-age=31536000, immutable", "font/woff2"},
		"/theme-init.js":       {"no-cache", "text/javascript"},
	}
	for target, want := range cases {
		rec := get(h, "GET", target)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", target, rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != want.cache {
			t.Errorf("GET %s: Cache-Control = %q, want %q", target, got, want.cache)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, want.contentType) {
			t.Errorf("GET %s: Content-Type = %q, want %q", target, got, want.contentType)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s: nosniff missing", target)
		}
	}
}

func TestConditionalRequestsReturn304(t *testing.T) {
	h := wrapped(t, builtFiles())
	first := get(h, "GET", "/")
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the shell")
	}
	if rec := get(h, "GET", "/", "If-None-Match", etag); rec.Code != http.StatusNotModified {
		t.Errorf("conditional GET = %d, want 304", rec.Code)
	}
}

func TestMissingFilesAnd404(t *testing.T) {
	h := wrapped(t, builtFiles())
	for _, target := range []string{"/assets/gone.js", "/robots.txt", "/favicon.ico", "/openapi.yaml", "/x/y.map", "/.gitkeep", "/assets/"} {
		rec := get(h, "GET", target)
		if target == "/assets/" {
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s = %d, want the shell", target, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/json" || strings.Contains(rec.Body.String(), "<html") {
			t.Errorf("GET %s = %d %q %q", target, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
}

func TestAPIAndReservedPathsNeverGetTheShell(t *testing.T) {
	h := wrapped(t, builtFiles())
	for _, target := range []string{
		"/api", "/api/", "/api/v1/me", "/api/v1/nope/", "/s", "/s/site/", "/s/site/index.html",
		"/.well-known/anything", "//", "/./", "/a//b", "/a/../b", "/api//v1/me",
	} {
		for _, method := range []string{"GET", "HEAD"} {
			if rec := get(h, method, target); rec.Code != http.StatusTeapot {
				t.Errorf("%s %s = %d, want the next handler", method, target, rec.Code)
			}
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		for _, target := range []string{"/", "/login", "/assets/app-1.js"} {
			if rec := get(h, method, target); rec.Code != http.StatusTeapot {
				t.Errorf("%s %s = %d, want the next handler", method, target, rec.Code)
			}
		}
	}
}

func TestHeadReturnsHeadersWithoutBody(t *testing.T) {
	h := wrapped(t, builtFiles())
	rec := get(h, "HEAD", "/login")
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("Content-Type") == "" {
		t.Errorf("HEAD = %d, body %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestUnbuiltDashboardShowsAPlainTextPage(t *testing.T) {
	h := wrapped(t, fstest.MapFS{".gitkeep": {}})
	rec := get(h, "GET", "/login")
	if rec.Code != http.StatusServiceUnavailable || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("GET /login = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "pnpm --dir web build") || strings.Contains(rec.Body.String(), "<") {
		t.Errorf("body = %q", rec.Body.String())
	}
	if rec := get(h, "GET", "/assets/app.js"); rec.Code != http.StatusNotFound {
		t.Errorf("asset without a build = %d, want 404", rec.Code)
	}
	if rec := get(h, "GET", "/api/v1/me"); rec.Code != http.StatusTeapot {
		t.Errorf("API path without a build = %d", rec.Code)
	}
}

func TestEmbeddedPlaceholderCompilesAndReportsNotBuilt(t *testing.T) {
	files, err := Dist()
	if err != nil {
		t.Fatal(err)
	}
	shell, err := New(files)
	if err != nil {
		t.Fatal(err)
	}
	if shell.Built() {
		t.Skip("a real dashboard build is present")
	}
	if rec := get(shell.Wrap(apiAnswer), "GET", "/"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET / = %d", rec.Code)
	}
}
