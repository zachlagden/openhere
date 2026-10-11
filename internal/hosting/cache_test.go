package hosting

import (
	"context"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestRepublishChangesWhatIsServedImmediately(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "version one", "old.html": "old page"}})
		e.enableSubdomains()
		expectBody(t, e.site("/"), 200, "version one")
		expectBody(t, e.site("/old"), 200, "old page")
		expectBody(t, e.site("/new"), 404, "Not found\n")

		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "version two", "new.html": "new page"}})
		expectBody(t, e.site("/"), 200, "version two")
		expectBody(t, e.site("/new"), 200, "new page")
		expectBody(t, e.site("/old"), 404, "Not found\n")
		expectBody(t, e.get(managementHost, "/s/demo/"), 200, "version two")
	})
}

func TestRestoreChangesWhatIsServedImmediately(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		_, first := e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "version one"}})
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "version two"}})
		e.enableSubdomains()
		expectBody(t, e.site("/"), 200, "version two")

		if _, err := e.publish.Restore(context.Background(), e.alex, "demo", first); err != nil {
			t.Fatal(err)
		}
		expectBody(t, e.site("/"), 200, "version one")
	})
}

func TestDeletingASiteStopsServingItImmediately(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.enableSubdomains()
		expectBody(t, e.site("/"), 200, "<h1>demo</h1>")

		if err := e.sites.DeleteSite(context.Background(), e.alex, "demo"); err != nil {
			t.Fatal(err)
		}
		expectBody(t, e.site("/"), 404, "Not found\n")
		expectBody(t, e.get(managementHost, "/s/demo/"), 404, "Not found\n")
	})
}

func TestSPAModeChangeApplies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "shell"}})
		e.enableSubdomains()
		expectBody(t, e.site("/route"), 404, "Not found\n")

		on := true
		if _, err := e.sites.UpdateSite(context.Background(), e.alex, "demo", sitesUpdate(&on)); err != nil {
			t.Fatal(err)
		}
		expectBody(t, e.site("/route"), 200, "shell")
	})
}

func TestNewSiteAppearsAfterAnEarlierMiss(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.enableSubdomains()
		expectBody(t, e.get("fresh.sites.example.com", "/"), 404, "Not found\n")
		e.publishSite("fresh", demoSite)
		expectBody(t, e.get("fresh.sites.example.com", "/"), 200, "<h1>demo</h1>")
	})
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func TestResolverEntriesExpire(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		clock := &fakeClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
		resolver := NewResolver(db, clock.Now)

		if _, live, err := resolver.Lookup(t.Context(), "demo"); err != nil || live {
			t.Fatalf("before publishing: live = %v, %v", live, err)
		}
		e.publishSite("demo", demoSite)
		if _, live, _ := resolver.Lookup(t.Context(), "demo"); live {
			t.Error("the miss was not cached")
		}
		clock.now = clock.now.Add(missingTTL + time.Second)
		site, live, err := resolver.Lookup(t.Context(), "demo")
		if err != nil || !live || site.VersionID == "" {
			t.Fatalf("after the miss expired: %+v, %v, %v", site, live, err)
		}
		resolver.Invalidate("demo")
		if len(resolver.entries) != 0 {
			t.Errorf("Invalidate left %d entries", len(resolver.entries))
		}
	})
}

func TestResolverStaysBounded(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		resolver := NewResolver(db, nil)
		for i := 0; i < maxCachedKeys+200; i++ {
			if _, _, err := resolver.Lookup(t.Context(), "miss-"+itoa(i)); err != nil {
				t.Fatal(err)
			}
		}
		if len(resolver.entries) > maxCachedKeys {
			t.Errorf("cache holds %d entries, limit %d", len(resolver.entries), maxCachedKeys)
		}
	})
}

func TestInvalidationDuringALoadKeepsTheStaleResultOut(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		resolver := NewResolver(db, nil)
		resolver.mu.Lock()
		generation := resolver.generation
		resolver.mu.Unlock()
		resolver.Invalidate("demo")
		resolver.store("demo", Site{ID: "stale"}, true, generation)
		if len(resolver.entries) != 0 {
			t.Error("a result loaded before an invalidation was cached")
		}
	})
}
