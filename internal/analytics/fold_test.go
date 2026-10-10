package analytics

import (
	"context"
	"fmt"
	"net/netip"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func storedPathCounts(t *testing.T, db store.Store) map[string]int {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT path, COUNT(*) FROM analytics_events GROUP BY path")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for rows.Next() {
		var path string
		var n int
		if err := rows.Scan(&path, &n); err != nil {
			t.Fatal(err)
		}
		counts[path] = n
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	return counts
}

func recordPaths(t *testing.T, r *Recorder, siteID string, status int, paths ...string) {
	t.Helper()
	for i, p := range paths {
		hit := hitFor(siteID)
		hit.Path, hit.Status = p, status
		hit.Addr = netip.MustParseAddr(fmt.Sprintf("203.0.113.%d", i%250+1))
		r.Record(hit)
	}
	if err := r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDistinctPathsPerDayAreCapped(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{MaxPathsPerDay: 3})
		recordPaths(t, r, site.ID, 200, "/a", "/b", "/c", "/d", "/e", "/a")
		recordPaths(t, r, site.ID, 200, "/f", "/b")
		got := storedPathCounts(t, db)
		want := map[string]int{"/a": 2, "/b": 2, "/c": 1, PathOther: 3}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("stored paths = %v, want %v", got, want)
		}
	})
}

func TestTheCapIsPerSiteAndPerDay(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		first, second := e.site("first"), e.site("second")
		r := e.recorder(RecorderOptions{MaxPathsPerDay: 2})
		recordPaths(t, r, first.ID, 200, "/a", "/b", "/c")
		recordPaths(t, r, second.ID, 200, "/x", "/y")
		e.clock.Set(e.clock.Now().AddDate(0, 0, 1))
		recordPaths(t, r, first.ID, 200, "/c", "/d")
		var other int
		row := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM analytics_events WHERE path = $1", PathOther)
		if err := row.Scan(&other); err != nil || other != 1 {
			t.Errorf("folded events = %d, err %v; want only the third path of the first day", other, err)
		}
	})
}

func TestTheNotFoundBucketDoesNotUseUpTheCap(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{MaxPathsPerDay: 2})
		recordPaths(t, r, site.ID, 404, PathNotFound, PathNotFound, PathNotFound)
		recordPaths(t, r, site.ID, 200, "/a", "/b")
		got := storedPathCounts(t, db)
		if got[PathNotFound] != 3 || got["/a"] != 1 || got["/b"] != 1 || got[PathOther] != 0 {
			t.Errorf("stored paths = %v", got)
		}
	})
}

func TestEventsPerVisitorAreRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{VisitorPerMin: 3})
		for range 10 {
			r.Record(hitFor(site.ID))
		}
		other := hitFor(site.ID)
		other.Addr = netip.MustParseAddr("198.51.100.9")
		r.Record(other)
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := countEvents(t, db); got != 4 {
			t.Errorf("stored events = %d, want 3 from the flooding visitor and 1 from the other", got)
		}
		if got := r.Dropped(); got != 7 {
			t.Errorf("dropped = %d, want 7", got)
		}
	})
}

func TestBotEventsAreRateLimitedPerSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{BotPerMin: 2})
		for range 5 {
			hit := hitFor(site.ID)
			hit.UserAgent = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
			r.Record(hit)
		}
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := countEvents(t, db); got != 2 {
			t.Errorf("stored bot events = %d, want 2", got)
		}
	})
}

func TestRollupKeepsFoldedPathsAndKeepsNotFoundOutOfPages(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{MaxPathsPerDay: 2})
		recordPaths(t, r, site.ID, 200, "/a", "/b", "/c", "/d")
		recordPaths(t, r, site.ID, 404, PathNotFound, PathNotFound)
		e.clock.Set(e.clock.Now().AddDate(0, 0, 2))
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		var other, notFound, total int64
		rows := []struct {
			query string
			into  *int64
		}{
			{"SELECT COALESCE(SUM(views), 0) FROM analytics_daily WHERE dimension = 'path' AND value = '" + PathOther + "'", &other},
			{"SELECT COALESCE(SUM(views), 0) FROM analytics_daily WHERE dimension = 'path' AND value = '" + PathNotFound + "'", &notFound},
			{"SELECT COALESCE(SUM(views), 0) FROM analytics_daily WHERE dimension = 'status' AND value = '4xx'", &total},
		}
		for _, q := range rows {
			if err := db.QueryRowContext(context.Background(), q.query).Scan(q.into); err != nil {
				t.Fatal(err)
			}
		}
		if other != 2 || notFound != 0 || total != 2 {
			t.Errorf("rolled up other = %d, not found paths = %d, 4xx statuses = %d; want 2, 0, 2", other, notFound, total)
		}
	})
}
