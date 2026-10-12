package hosting

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

const hashedAsset = "assets/index-Bv3HVboL.js"

func TestCacheControlFor(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		filePath    string
		private     bool
		want        string
	}{
		{"hashed under assets", "text/javascript", "assets/index-Bv3HVboL.js", false, CacheImmutable},
		{"hashed under _next", "text/javascript", "_next/static/chunks/main-app-a1b2c3d4e5f6a7b8.js", false, CacheImmutable},
		{"hashed under static", "text/css", "static/css/main.8a1b2c3d.css", false, CacheImmutable},
		{"hashed under build", "font/woff2", "build/app_9f8e7d6c5b.woff2", false, CacheImmutable},
		{"ordinary file", "text/css", "assets/app.css", false, CacheFile},
		{"hashed outside the roots", "text/javascript", "main.8a1b2c3d.js", false, CacheFile},
		{"html", "text/html; charset=utf-8", "index.html", false, CacheFile},
		{"hashed html", "text/html; charset=utf-8", "assets/page-8a1b2c3d.html", false, CacheFile},
		{"gated hashed", "text/javascript", "assets/index-Bv3HVboL.js", true, cachePrivateFile},
		{"gated ordinary", "text/css", "assets/app.css", true, cachePrivateFile},
		{"gated html", "text/html; charset=utf-8", "index.html", true, cachePrivateFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cacheControlFor(tc.contentType, tc.filePath, tc.private); got != tc.want {
				t.Errorf("cacheControlFor = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNoSharedCacheDirectiveIsEverSent(t *testing.T) {
	for _, value := range []string{CacheFile, CacheImmutable, cachePrivateFile, cacheGatePage} {
		if strings.Contains(value, "public") || strings.Contains(value, "s-maxage") {
			t.Errorf("%q lets a shared cache store the response", value)
		}
	}
}

func assertNotSharedCacheable(t *testing.T, label string, rec interface{ Header() http.Header }) {
	t.Helper()
	value := rec.Header().Get("Cache-Control")
	if value == "" {
		t.Errorf("%s: Cache-Control is empty", label)
	}
	if strings.Contains(value, "public") || strings.Contains(value, "s-maxage") {
		t.Errorf("%s: Cache-Control = %q", label, value)
	}
}

func TestHashedAssetResponsesCarryTheImmutablePrivatePolicy(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{
			"index.html":  "<p>",
			hashedAsset:   "0123456789",
			"404.html":    "custom missing",
			"assets/a.js": "plain",
		}})
		e.enableSubdomains()

		places := []struct{ name, host, prefix string }{
			{"subdomain", siteHost, ""},
			{"path mode", managementHost, "/s/demo"},
		}
		for _, place := range places {
			target := place.prefix + "/" + hashedAsset

			rec := e.get(place.host, target)
			if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != CacheImmutable {
				t.Errorf("%s 200: %d %q", place.name, rec.Code, rec.Header().Get("Cache-Control"))
			}
			etag := rec.Header().Get("ETag")

			partial := e.get(place.host, target, "Range", "bytes=2-5")
			if partial.Code != http.StatusPartialContent || partial.Body.String() != "2345" || partial.Header().Get("Cache-Control") != CacheImmutable {
				t.Errorf("%s 206: %d %q %q", place.name, partial.Code, partial.Body.String(), partial.Header().Get("Cache-Control"))
			}

			notModified := e.get(place.host, target, "If-None-Match", etag)
			if notModified.Code != http.StatusNotModified || notModified.Header().Get("Cache-Control") != CacheImmutable || notModified.Header().Get("ETag") != etag {
				t.Errorf("%s 304: %d %v", place.name, notModified.Code, notModified.Header())
			}

			head := e.request(http.MethodHead, place.host, target)
			if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Cache-Control") != CacheImmutable {
				t.Errorf("%s HEAD: %d %q", place.name, head.Code, head.Header().Get("Cache-Control"))
			}

			missing := e.get(place.host, place.prefix+"/assets/gone-Bv3HVboL.js")
			if missing.Code != http.StatusNotFound || missing.Header().Get("Cache-Control") != CacheFile {
				t.Errorf("%s custom 404: %d %q", place.name, missing.Code, missing.Header().Get("Cache-Control"))
			}
			assertNotSharedCacheable(t, place.name+" custom 404", missing)

			wrongMethod := e.request(http.MethodPost, place.host, target)
			if wrongMethod.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s POST: %d", place.name, wrongMethod.Code)
			}
			assertNotSharedCacheable(t, place.name+" 405", wrongMethod)
		}

		redirect := e.get(managementHost, "/s/demo/assets")
		if redirect.Code != http.StatusMovedPermanently && redirect.Code != http.StatusNotFound {
			t.Errorf("directory request: %d", redirect.Code)
		}
		assertNotSharedCacheable(t, "directory request", redirect)
	})
}

func TestGenericNotFoundAndRedirectsAreNotSharedCacheable(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "<p>", "docs/index.html": "docs"}})
		e.enableSubdomains()

		missing := e.site("/missing")
		if missing.Code != http.StatusNotFound || missing.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("generic 404: %d %q", missing.Code, missing.Header().Get("Cache-Control"))
		}
		redirect := e.site("/docs")
		if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("directory redirect: %d %q", redirect.Code, redirect.Header().Get("Cache-Control"))
		}
	})
}

func TestHashedAssetStaysUnreadableAfterTheSiteTurnsPrivate(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "<p>home</p>", hashedAsset: "console.log(1)"}})
		e.enableSubdomains()
		target := "/" + hashedAsset

		before := e.site(target)
		if before.Code != http.StatusOK || before.Body.String() != "console.log(1)" {
			t.Fatalf("public fetch = %d %q", before.Code, before.Body.String())
		}
		assertNotSharedCacheable(t, "public fetch", before)

		e.protect("demo", passwordMode())

		denied := e.site(target)
		if denied.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous fetch after protect = %d", denied.Code)
		}
		assertNoContent(t, "anonymous fetch after protect", denied.Body.String())
		if denied.Header().Get("Cache-Control") != cacheGatePage {
			t.Errorf("401 Cache-Control = %q", denied.Header().Get("Cache-Control"))
		}
		revalidated := e.site(target, "If-None-Match", before.Header().Get("ETag"))
		if revalidated.Code != http.StatusUnauthorized {
			t.Errorf("conditional fetch after protect = %d", revalidated.Code)
		}

		cookie := e.unlockedCookie(siteHost, "")
		allowed := e.withCookie(siteHost, target, cookie)
		if allowed.Code != http.StatusOK || allowed.Body.String() != "console.log(1)" {
			t.Fatalf("viewer fetch = %d %q", allowed.Code, allowed.Body.String())
		}
		if allowed.Header().Get("Cache-Control") != cachePrivateFile {
			t.Errorf("viewer Cache-Control = %q", allowed.Header().Get("Cache-Control"))
		}
		assertNotSharedCacheable(t, "viewer fetch", allowed)
	})
}
