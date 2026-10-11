package analytics

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func hitFor(siteID string) Hit {
	return Hit{
		SiteID: siteID, Path: "/docs", Status: 200, Referrer: "https://news.example.org/story?id=1", Host: "demo.example.com",
		Addr: netip.MustParseAddr(docIP), UserAgent: docUA,
	}
}

func countEvents(t *testing.T, db store.Store) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM analytics_events").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRecorderStoresParsedFields(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{Locator: fakeLocator{country: "GB", city: "London"}})
		r.Record(hitFor(site.ID))
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		var got Event
		var visitor *string
		err := db.QueryRowContext(context.Background(),
			"SELECT site_id, path, status_class, referrer_host, country, city, browser, os, device, visitor FROM analytics_events").
			Scan(&got.SiteID, &got.Path, &got.StatusClass, &got.ReferrerHost, &got.Country, &got.City, &got.Browser, &got.OS, &got.Device, &visitor)
		if err != nil {
			t.Fatal(err)
		}
		want := Event{
			SiteID: site.ID, Path: "/docs", StatusClass: "2xx", ReferrerHost: "news.example.org", Country: "GB", City: "London",
			Browser: "Firefox", OS: "Linux", Device: DeviceDesktop,
		}
		got.Visitor = ""
		if got != want {
			t.Errorf("event = %+v, want %+v", got, want)
		}
		if visitor == nil || len(*visitor) != 32 {
			t.Errorf("visitor = %v", visitor)
		}
	})
}

func TestRecorderKeepsBotsOutOfUniques(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{Locator: fakeLocator{country: "GB", city: "London"}})
		hit := hitFor(site.ID)
		hit.UserAgent = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
		r.Record(hit)
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		var device string
		var visitor, country *string
		err := db.QueryRowContext(context.Background(), "SELECT device, visitor, country FROM analytics_events").Scan(&device, &visitor, &country)
		if err != nil {
			t.Fatal(err)
		}
		if device != DeviceBot || visitor != nil || (country != nil && *country != "") {
			t.Errorf("device = %q, visitor = %v, country = %v", device, visitor, country)
		}
	})
}

func TestRecorderStoresNoIPNoRawUserAgentNoQuery(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{})
		hit := hitFor(site.ID)
		hit.Path = "/docs/page?token=secret-token-123&utm=x#frag"
		hit.Referrer = "https://news.example.org/story?session=abc123"
		r.Record(hit)
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		dump := dumpTables(t, db)
		for _, forbidden := range []string{docIP, "192.0.2", docUA, "Gecko/20100101", "token=", "secret-token-123", "utm=", "session=abc123", "story", "frag"} {
			if strings.Contains(dump, forbidden) {
				t.Errorf("the database contains %q:\n%s", forbidden, dump)
			}
		}
		if !strings.Contains(dump, "/docs/page") || !strings.Contains(dump, "news.example.org") {
			t.Errorf("the dump is missing the expected fields:\n%s", dump)
		}
	})
}

func dumpTables(t *testing.T, db store.Store) string {
	t.Helper()
	var out strings.Builder
	for _, query := range []string{
		"SELECT site_id, occurred_at, day, hour, path, status_class, referrer_host, country, city, browser, os, device, COALESCE(visitor, '') FROM analytics_events",
		"SELECT site_id, day, dimension, value, views, uniques FROM analytics_daily",
		"SELECT key, value FROM instance_settings",
	} {
		rows, err := db.QueryContext(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for _, v := range values {
				out.WriteString(toText(v))
				out.WriteString("|")
			}
			out.WriteString("\n")
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return out.String()
}

func toText(v any) string {
	switch typed := v.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	case time.Time:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func TestRecorderDropsWhenTheBufferIsFull(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := NewRecorder(db, RecorderOptions{Hasher: e.hasher, Logger: discardLogger(), BufferSize: 3})
		for range 10 {
			r.Record(hitFor(site.ID))
		}
		if r.Dropped() != 7 {
			t.Fatalf("dropped = %d, want 7", r.Dropped())
		}
		r.Start()
		if err := r.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := countEvents(t, db); n != 3 {
			t.Errorf("stored %d events, want the 3 that fitted", n)
		}
	})
}

func TestRecorderDoesNotBlockWhileTheWorkerIsStuck(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := NewRecorder(db, RecorderOptions{Hasher: e.hasher, Logger: discardLogger(), BufferSize: 2})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for range 1000 {
				r.Record(hitFor(site.ID))
			}
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Record blocked")
		}
	})
}

func TestCloseFlushesPendingEvents(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := NewRecorder(db, RecorderOptions{Hasher: e.hasher, Logger: discardLogger(), FlushInterval: time.Hour})
		r.Start()
		for range 25 {
			r.Record(hitFor(site.ID))
		}
		if err := r.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := countEvents(t, db); n != 25 {
			t.Errorf("stored %d events after Close, want 25", n)
		}
		r.Record(hitFor(site.ID))
		if r.Dropped() != 1 {
			t.Errorf("a hit after Close was not counted as dropped: %d", r.Dropped())
		}
		if err := r.Close(context.Background()); err != nil {
			t.Errorf("second Close = %v", err)
		}
	})
}

func TestRecorderBatchesByCountAndByTimer(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := NewRecorder(db, RecorderOptions{Hasher: e.hasher, Logger: discardLogger(), BatchSize: 4, FlushInterval: time.Hour})
		r.Start()
		t.Cleanup(func() {
			if err := r.Close(context.Background()); err != nil {
				t.Errorf("close: %v", err)
			}
		})
		for range 9 {
			r.Record(hitFor(site.ID))
		}
		waitFor(t, func() bool { return countEvents(t, db) == 8 })
		if n := countEvents(t, db); n != 8 {
			t.Errorf("after 9 hits and a batch size of 4, %d events were written, want 8", n)
		}

		timed := NewRecorder(db, RecorderOptions{Hasher: e.hasher, Logger: discardLogger(), FlushInterval: 20 * time.Millisecond})
		timed.Start()
		t.Cleanup(func() {
			if err := timed.Close(context.Background()); err != nil {
				t.Errorf("close: %v", err)
			}
		})
		timed.Record(hitFor(site.ID))
		waitFor(t, func() bool { return countEvents(t, db) == 9 })
	})
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestRecorderDropsOnlyTheRowsThatFail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{})
		r.Record(hitFor(site.ID))
		r.Record(hitFor("site-that-does-not-exist"))
		r.Record(hitFor(site.ID))
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := countEvents(t, db); n != 2 {
			t.Errorf("stored %d events, want 2", n)
		}
		if r.Dropped() != 1 {
			t.Errorf("dropped = %d, want 1", r.Dropped())
		}
	})
}

func TestRecorderIgnoresUncountedStatuses(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		r := e.recorder(RecorderOptions{})
		for _, status := range []int{301, 304, 403, 500} {
			hit := hitFor(site.ID)
			hit.Status = status
			r.Record(hit)
		}
		notFound := hitFor(site.ID)
		notFound.Status = 404
		r.Record(notFound)
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := countEvents(t, db); n != 1 {
			t.Errorf("stored %d events, want only the 404", n)
		}
		if r.Dropped() != 0 {
			t.Errorf("uncounted statuses were reported as drops: %d", r.Dropped())
		}
	})
}

func TestRecorderDropsEventsOfDeletedSitesQuietly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		kept, gone, alsoKept := e.site("kept"), e.site("gone"), e.site("also-kept")
		logs := &bytes.Buffer{}
		r := NewRecorder(db, RecorderOptions{
			Hasher: e.hasher, Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})), Now: e.clock.Now,
		})
		r.Start()
		t.Cleanup(func() {
			if err := r.Close(context.Background()); err != nil {
				t.Errorf("close recorder: %v", err)
			}
		})

		for _, id := range []string{kept.ID, gone.ID, alsoKept.ID, gone.ID, kept.ID} {
			r.Record(hitFor(id))
		}
		if err := e.sites.DeleteSite(context.Background(), e.alex, "gone"); err != nil {
			t.Fatal(err)
		}
		if err := r.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}

		if n := countEvents(t, db); n != 3 {
			t.Errorf("stored %d events, want 3", n)
		}
		if r.Dropped() != 2 {
			t.Errorf("dropped = %d, want 2", r.Dropped())
		}
		out := logs.String()
		if strings.Contains(out, "level=WARN") || strings.Contains(out, "level=ERROR") {
			t.Errorf("a deleted site logged above debug:\n%s", out)
		}
		if !strings.Contains(out, "level=DEBUG") || !strings.Contains(out, "events=2") {
			t.Errorf("missing the debug count:\n%s", out)
		}
		if strings.Contains(out, gone.ID) {
			t.Error("the log names a site id")
		}
	})
}
