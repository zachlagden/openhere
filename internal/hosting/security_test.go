package hosting

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

const wantSandbox = "sandbox allow-scripts allow-forms allow-popups allow-downloads allow-modals allow-presentation"

func TestPathModeSendsTheSandboxOnEveryResponse(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		targets := []string{
			"/s/demo/", "/s/demo/about", "/s/demo/index.html", "/s/demo/_next/static/chunk-1.js", "/s/demo/data.json",
			"/s/demo/docs", "/s/demo", "/s/demo/missing", "/s/nobody/", "/s/nobody", "/s/", "/s", "/s/Bad_Slug/", "/s/demo/%2e%2e/x",
		}
		for _, target := range targets {
			rec := e.get(managementHost, target)
			assertSandboxed(t, "GET "+target, rec.Header())
		}
		for _, method := range []string{"HEAD", "POST", "DELETE"} {
			assertSandboxed(t, method, e.request(method, managementHost, "/s/demo/").Header())
		}
		assertSandboxed(t, "304", e.get(managementHost, "/s/demo/", "If-None-Match", `"`+hashOf("home")+`"`).Header())
		assertSandboxed(t, "range", e.get(managementHost, "/s/demo/data.json", "Range", "bytes=0-2").Header())
		assertSandboxed(t, "bad range", e.get(managementHost, "/s/demo/data.json", "Range", "bytes=99-").Header())

		e.set(`{"hosting.pathEnabled": false}`)
		assertSandboxed(t, "disabled", e.get(managementHost, "/s/demo/").Header())
	})
}

func assertSandboxed(t *testing.T, label string, h http.Header) {
	t.Helper()
	if got := h.Get("Content-Security-Policy"); got != wantSandbox {
		t.Errorf("%s: Content-Security-Policy = %q", label, got)
	}
	if strings.Contains(h.Get("Content-Security-Policy"), "allow-same-origin") {
		t.Errorf("%s: the sandbox must not allow the same origin", label)
	}
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("%s: nosniff is missing", label)
	}
}

func TestSubdomainModeImposesNoPolicy(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		for _, target := range []string{"/", "/about", "/missing", "/docs", "/_next/static/chunk-1.js"} {
			h := e.site(target).Header()
			if h.Get("Content-Security-Policy") != "" || h.Get("Access-Control-Allow-Origin") != "" {
				t.Errorf("%s: subdomain mode set a policy: %v", target, h)
			}
		}
		if e.get("nobody.sites.example.com", "/").Header().Get("Content-Security-Policy") != "" {
			t.Error("a subdomain 404 carries a CSP")
		}
	})
}

func TestTraversalAttemptsNeverLeaveTheSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		siteID, versionID := e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "home", "404.html": "custom missing"}})
		outside := filepath.Join(e.dataDir, "secret.txt")
		if err := os.WriteFile(outside, []byte("TOP SECRET"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(e.versionDir(siteID, versionID), "..", "sibling.txt"), []byte("TOP SECRET"), 0o600); err != nil {
			t.Fatal(err)
		}
		e.publishSite("other", siteSpec{files: map[string]string{"index.html": "other home", "private.txt": "OTHER SITE"}})
		e.enableSubdomains()

		attempts := []string{
			"/../secret.txt", "/..%2fsecret.txt", "/%2e%2e/secret.txt", "/%2e%2e%2fsecret.txt", "/%2E%2E/%2E%2E/secret.txt",
			"/a/../../secret.txt", "/..%5csecret.txt", "/..\\secret.txt", "/%2e%2e%5csecret.txt", "/....//secret.txt",
			"/%252e%252e/secret.txt", "/index.html/../../../secret.txt", "/../sibling.txt", "/../../sites/", "/%00", "/index.html%00.png",
			"/./index.html/../../secret.txt", "/.%2e/secret.txt", "/%c0%ae%c0%ae/secret.txt", "/..;/secret.txt",
			"/../other/private.txt", "/.. /secret.txt", "/%2e%2e", "/..", "/../",
		}
		for _, target := range attempts {
			for _, base := range []string{"", "/s/demo"} {
				host, prefix := siteHost, ""
				if base != "" {
					host, prefix = managementHost, base
				}
				rec, parsed := e.rawGet(host, prefix+target)
				if !parsed {
					continue
				}
				body := rec.Body.String()
				if strings.Contains(body, "TOP SECRET") || strings.Contains(body, "OTHER SITE") {
					t.Errorf("GET %s%s leaked a file: %q", prefix, target, body)
				}
				if rec.Code == 200 && !strings.Contains(target, "index.html") && target != "/.." && target != "/../" && target != "/%2e%2e" {
					t.Errorf("GET %s%s = 200 %q, want a refusal", prefix, target, body)
				}
			}
		}
	})
}

func TestRawRequestTargetsAreContained(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "home"}})
		e.enableSubdomains()

		for _, raw := range []string{"/s/demo/../../api", "/s/../s/demo/", "/s/demo//", "/s/demo/./", "/s/demo/%2e/"} {
			rec, parsed := e.rawGet(managementHost, raw)
			if !parsed {
				t.Fatalf("%s did not parse", raw)
			}
			if strings.Contains(rec.Body.String(), "management") && rec.Header().Get("X-Management") != "" && strings.HasPrefix(raw, "/s/demo") {
				t.Errorf("%s escaped path mode into the management handler", raw)
			}
		}
	})
}

func TestSymlinksAreRefused(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		siteID, versionID := e.publishSite("demo", siteSpec{files: map[string]string{
			"index.html": "home", "link.txt": "placeholder", "docs/index.html": "docs", "404.html": "custom missing",
		}})
		e.enableSubdomains()
		secret := filepath.Join(e.dataDir, "secret.txt")
		if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o600); err != nil {
			t.Fatal(err)
		}
		dir := e.versionDir(siteID, versionID)
		writeThroughSymlink(t, dir, "link.txt", secret)
		writeThroughSymlink(t, dir, "docs/index.html", secret)

		for _, target := range []string{"/link.txt", "/s/demo/link.txt", "/docs/"} {
			host := siteHost
			if strings.HasPrefix(target, "/s/") {
				host = managementHost
			}
			rec := e.get(host, target)
			if strings.Contains(rec.Body.String(), "TOP SECRET") {
				t.Fatalf("GET %s followed a symlink", target)
			}
			if rec.Code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404", target, rec.Code)
			}
		}
		expectBody(t, e.site("/"), 200, "home")
	})
}

func TestSymlinkedDirectoryIsRefused(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		siteID, versionID := e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "home", "assets/app.js": "js"}})
		e.enableSubdomains()
		outsideDir := filepath.Join(e.dataDir, "outside")
		if err := os.MkdirAll(outsideDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outsideDir, "app.js"), []byte("TOP SECRET"), 0o600); err != nil {
			t.Fatal(err)
		}
		dir := e.versionDir(siteID, versionID)
		if err := os.RemoveAll(filepath.Join(dir, "assets")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideDir, filepath.Join(dir, "assets")); err != nil {
			t.Fatal(err)
		}
		rec := e.site("/assets/app.js")
		if strings.Contains(rec.Body.String(), "TOP SECRET") || rec.Code != http.StatusNotFound {
			t.Errorf("GET through a symlinked directory = %d %q", rec.Code, rec.Body.String())
		}
	})
}

func TestUnlistedFilesOnDiskAreNotServed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		siteID, versionID := e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "home"}})
		e.enableSubdomains()
		if err := os.WriteFile(filepath.Join(e.versionDir(siteID, versionID), "extra.txt"), []byte("UNLISTED"), 0o600); err != nil {
			t.Fatal(err)
		}
		expectBody(t, e.site("/extra.txt"), 404, "Not found\n")
	})
}
