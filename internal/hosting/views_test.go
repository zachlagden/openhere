package hosting

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/zachlagden/openhere/internal/analytics"
	"github.com/zachlagden/openhere/internal/proxy"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

type viewLog struct {
	mu   sync.Mutex
	hits []analytics.Hit
}

func (v *viewLog) Record(hit analytics.Hit) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hits = append(v.hits, hit)
}

func (v *viewLog) all() []analytics.Hit {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]analytics.Hit(nil), v.hits...)
}

func (v *viewLog) count() int { return len(v.all()) }

func (v *viewLog) reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hits = nil
}

func expectViews(t *testing.T, e *env, want int, what string) {
	t.Helper()
	if got := e.views.count(); got != want {
		t.Errorf("%s: %d views recorded, want %d", what, got, want)
	}
}

func TestHTMLPagesRecordAView(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()
		rec := e.site("/about?utm=x&token=abc", "Referer", "https://news.example.org/story?id=1", "User-Agent", "Mozilla/5.0 Firefox/127.0")
		expectBody(t, rec, 200, "about")
		hits := e.views.all()
		if len(hits) != 1 {
			t.Fatalf("%d views recorded, want 1", len(hits))
		}
		hit := hits[0]
		if hit.SiteID != e.siteIDOf("demo") || hit.Path != "/about" || hit.Status != 200 || hit.Host != siteHost {
			t.Errorf("hit = %+v", hit)
		}
		if hit.Referrer != "https://news.example.org/story?id=1" || hit.UserAgent != "Mozilla/5.0 Firefox/127.0" {
			t.Errorf("hit = %+v", hit)
		}
	})
}

func TestPathModeAndCustomDomainsRecordViews(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.verifiedDomain("demo", customHost)
		expectBody(t, e.get(managementHost, "/s/demo/docs/"), 200, "docs index")
		expectBody(t, e.get(customHost, "/blog"), 200, "blog page")
		hits := e.views.all()
		if len(hits) != 2 || hits[0].Path != "/docs/" || hits[1].Path != "/blog" {
			t.Errorf("hits = %+v", hits)
		}
	})
}

func TestSPAFallbackRecordsTheRootPath(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("spa-demo", siteSpec{files: map[string]string{"index.html": "shell"}, spa: true})
		e.enableSubdomains()
		expectBody(t, e.get("spa-demo.sites.example.com", "/dashboard/settings?tab=1"), 200, "shell")
		hits := e.views.all()
		if len(hits) != 1 || hits[0].Path != "/" || hits[0].Status != 200 {
			t.Errorf("hits = %+v", hits)
		}
	})
}

func TestCustomNotFoundPageRecordsA404(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()
		expectBody(t, e.site("/nope"), 404, "custom missing")
		hits := e.views.all()
		if len(hits) != 1 || hits[0].Path != "/(not found)" || hits[0].Status != 404 {
			t.Errorf("hits = %+v", hits)
		}
	})
}

func TestSitesWithoutANotFoundPageRecordNothingForMisses(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("plain", siteSpec{files: map[string]string{"index.html": "home"}})
		e.enableSubdomains()
		expectBody(t, e.get("plain.sites.example.com", "/nope"), 404, "Not found\n")
		expectViews(t, e, 0, "a bare 404")
	})
}

func TestUncountedRequestsRecordNothing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		for _, target := range []string{"/_next/static/chunk-1.js", "/_next/static/style.css", "/data.json", "/file.txt", "/favicon.ico"} {
			e.site(target)
		}
		expectViews(t, e, 0, "assets")

		if rec := e.request("HEAD", siteHost, "/about"); rec.Code != 200 {
			t.Fatalf("HEAD status = %d", rec.Code)
		}
		expectViews(t, e, 0, "HEAD")

		expectBody(t, e.site("/about", "Range", "bytes=0-1"), 206, "ab")
		expectViews(t, e, 0, "Range")

		etag := e.site("/about").Header().Get("ETag")
		e.views.reset()
		if rec := e.site("/about", "If-None-Match", etag); rec.Code != 304 {
			t.Fatalf("conditional status = %d", rec.Code)
		}
		expectViews(t, e, 0, "304")

		e.site("/about", "Sec-Purpose", "prefetch;prerender")
		e.site("/about", "Purpose", "prefetch")
		expectViews(t, e, 0, "prefetch")

		e.request("POST", siteHost, "/about")
		expectViews(t, e, 0, "POST")
	})
}

func TestTurningAnalyticsOffStopsRecording(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()
		e.site("/about")
		expectViews(t, e, 1, "analytics on")
		off := false
		if _, err := e.sites.UpdateSite(context.Background(), e.alex, "demo", analyticsUpdate(&off)); err != nil {
			t.Fatal(err)
		}
		e.site("/about")
		e.site("/nope")
		expectViews(t, e, 1, "analytics off")
		on := true
		if _, err := e.sites.UpdateSite(context.Background(), e.alex, "demo", analyticsUpdate(&on)); err != nil {
			t.Fatal(err)
		}
		e.site("/about")
		expectViews(t, e, 2, "analytics back on")
	})
}

func TestGatedSitesRecordOnlyAfterTheGateLetsTheRequestThrough(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		for _, target := range []string{"/", "/about", "/missing", "/_next/static/chunk-1.js"} {
			e.site(target)
		}
		e.site("/about", "Sec-Fetch-Mode", "navigate")
		expectViews(t, e, 0, "anonymous visitors of a gated site")

		cookie := e.unlockedCookie(siteHost, "")
		e.views.reset()
		expectBody(t, e.withCookie(siteHost, "/about", cookie), 200, "about")
		expectViews(t, e, 1, "an unlocked visitor")
	})
}

func TestTheRecordedAddressFollowsTheTrustedProxyRules(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		trusted := proxy.Trust{Prefixes: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}}
		e := newEnvWith(t, db, envOptions{proxies: trusted})
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		send := func(remote, forwarded string) analytics.Hit {
			e.views.reset()
			req := newSiteRequest(t, siteHost, "/about", remote, forwarded)
			e.router.ServeHTTP(newRecorder(), req)
			hits := e.views.all()
			if len(hits) != 1 {
				t.Fatalf("%d views recorded, want 1", len(hits))
			}
			return hits[0]
		}
		if got := send("203.0.113.5:4000", "192.0.2.44").Addr; got != netip.MustParseAddr("192.0.2.44") {
			t.Errorf("behind a trusted proxy the address was %v", got)
		}
		if got := send("198.51.100.9:4000", "192.0.2.44").Addr; got != netip.MustParseAddr("198.51.100.9") {
			t.Errorf("a forged header from an untrusted client gave %v", got)
		}
	})
}

func TestTheRecordedAddressFollowsTheClientIPHeaderFromTrustedProxies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		trusted := proxy.Trust{
			Prefixes:       []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
			ClientIPHeader: "Cf-Connecting-Ip",
		}
		e := newEnvWith(t, db, envOptions{proxies: trusted})
		e.publishSite("demo", nextExport)
		e.enableSubdomains()

		send := func(remote string) netip.Addr {
			e.views.reset()
			req := newSiteRequest(t, siteHost, "/about", remote, "172.69.1.1")
			req.Header.Set("Cf-Connecting-Ip", "192.0.2.44")
			e.router.ServeHTTP(newRecorder(), req)
			hits := e.views.all()
			if len(hits) != 1 {
				t.Fatalf("%d views recorded, want 1", len(hits))
			}
			return hits[0].Addr
		}
		if got := send("203.0.113.5:4000"); got != netip.MustParseAddr("192.0.2.44") {
			t.Errorf("behind a trusted proxy the address was %v", got)
		}
		if got := send("198.51.100.9:4000"); got != netip.MustParseAddr("198.51.100.9") {
			t.Errorf("a forged header from an untrusted client gave %v", got)
		}
	})
}

func TestDistinctMissingPathsShareOneRecordedPath(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.publishSite("spa-demo", siteSpec{files: map[string]string{"index.html": "shell"}, spa: true})
		e.enableSubdomains()
		for i := range 600 {
			e.site(fmt.Sprintf("/missing-%d", i))
			e.get("spa-demo.sites.example.com", fmt.Sprintf("/app/route-%d", i))
		}
		paths := map[string]int{}
		for _, hit := range e.views.all() {
			paths[hit.Path]++
		}
		if len(paths) != 2 || paths["/(not found)"] != 600 || paths["/"] != 600 {
			t.Errorf("recorded paths = %v, want 600 hits each under /(not found) and /", paths)
		}
	})
}
