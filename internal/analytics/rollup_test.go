package analytics

import (
	"context"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func dailyDump(t *testing.T, db store.Store) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(),
		"SELECT site_id, day, dimension, value, views, uniques FROM analytics_daily ORDER BY site_id, day, dimension, value")
	if err != nil {
		t.Fatal(err)
	}
	var out string
	for rows.Next() {
		var site, d, dim, value string
		var views, uniques int64
		if err := rows.Scan(&site, &d, &dim, &value, &views, &uniques); err != nil {
			t.Fatal(err)
		}
		out += site + "|" + d + "|" + dim + "|" + value + "|" + itoa(views) + "|" + itoa(uniques) + "\n"
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

func itoa(n int64) string {
	digits := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestRollupIsIdempotent(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		first := dailyDump(t, db)
		if first == "" {
			t.Fatal("nothing was rolled up")
		}
		for range 2 {
			if err := e.svc.Rollup(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		if second := dailyDump(t, db); second != first {
			t.Errorf("a second rollup changed the table:\n%s\nwant\n%s", second, first)
		}
	})
}

func TestRollupLeavesTodayAlone(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		var today int
		err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM analytics_daily WHERE day = $1", "2026-03-10").Scan(&today)
		if err != nil || today != 0 {
			t.Errorf("today has %d rollup rows, err = %v", today, err)
		}
	})
}

func TestReportsDoNotDoubleCountAcrossTheRollupBoundary(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		params := rangeParams("2026-03-08", "2026-03-10")
		checkFixedReport(t, e.report("demo", params))

		e.clock.Set(at("2026-03-09T12:00:00Z"))
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		checkFixedReport(t, e.report("demo", params))

		e.clock.Set(at("2026-03-11T12:00:00Z"))
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		checkFixedReport(t, e.report("demo", params))
	})
}

func TestRolledUpDataSurvivesRawDeletion(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		e.clock.Set(at("2026-04-20T12:00:00Z"))
		if err := e.svc.Maintain(context.Background()); err != nil {
			t.Fatal(err)
		}
		var raw int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM analytics_events").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if raw != 0 {
			t.Errorf("%d raw events are older than the raw window", raw)
		}
		checkFixedReport(t, e.report("demo", rangeParams("2026-03-08", "2026-03-10")))
	})
}

func TestRawEventsAreKeptUntilTheyAreRolledUp(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		e.clock.Set(at("2026-04-20T12:00:00Z"))
		if err := e.svc.Purge(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := countEvents(t, db); n != len(fixedData(site.ID)) {
			t.Errorf("purge removed unrolled events: %d left", n)
		}
	})
}

func TestLateEventsAreFoldedIntoARolledDay(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		e.clock.Set(at("2026-03-11T12:00:00Z"))
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		e.insert(view(site.ID, "2026-03-08T23:59:00Z", "/late", "h9"))
		params := rangeParams("2026-03-08", "2026-03-10")
		if got := e.report("demo", params).Totals.Views; got != 8 {
			t.Errorf("before the next rollup views = %d, want 8", got)
		}
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		report := e.report("demo", params)
		if report.Totals.Views != 9 || report.Totals.Uniques != 8 {
			t.Errorf("after the next rollup totals = %+v", report.Totals)
		}
	})
}

func TestRollupKeepsSitesApart(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		a, b := e.site("one"), e.site("two")
		e.insert(fixedData(a.ID)...)
		e.insert(view(b.ID, "2026-03-08T10:00:00Z", "/only-two", "k1"))
		e.clock.Set(at("2026-03-11T12:00:00Z"))
		if err := e.svc.Rollup(context.Background()); err != nil {
			t.Fatal(err)
		}
		checkFixedReport(t, e.report("one", rangeParams("2026-03-08", "2026-03-10")))
		other := e.report("two", rangeParams("2026-03-08", "2026-03-10"))
		if other.Totals.Views != 1 || len(other.Paths) != 1 || other.Paths[0].Value != "/only-two" {
			t.Errorf("second site report = %+v", other)
		}
	})
}

func TestRetentionDeletesOldRows(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(view(site.ID, "2025-01-05T10:00:00Z", "/old", "o1"), view(site.ID, "2026-03-08T10:00:00Z", "/new", "n1"))
		ctx := context.Background()
		e.clock.Set(at("2026-03-10T12:00:00Z"))
		if err := e.svc.Rollup(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := e.settings.Update(ctx, patchJSON(t, `{"analytics.retentionDays": 0}`)); err != nil {
			t.Fatal(err)
		}
		if err := e.svc.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if got := e.report("demo", rangeParams("2025-01-01", "2025-12-31")).Totals.Views; got != 1 {
			t.Errorf("with retention 0 the old rollup was removed: views = %d", got)
		}
		if _, err := e.settings.Update(ctx, patchJSON(t, `{"analytics.retentionDays": 30}`)); err != nil {
			t.Fatal(err)
		}
		if err := e.svc.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if got := e.report("demo", rangeParams("2025-01-01", "2025-12-31")).Totals.Views; got != 0 {
			t.Errorf("old views = %d, want 0", got)
		}
		if got := e.report("demo", rangeParams("2026-03-01", "2026-03-10")).Totals.Views; got != 1 {
			t.Errorf("recent views = %d, want 1", got)
		}
		var events, daily int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM analytics_events WHERE day < '2026-02-08'").Scan(&events); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM analytics_daily WHERE day < '2026-02-08'").Scan(&daily); err != nil {
			t.Fatal(err)
		}
		if events != 0 || daily != 0 {
			t.Errorf("rows older than the cutoff remain: %d events, %d rollups", events, daily)
		}
	})
}
