package analytics

import (
	"context"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func fixedData(site string) []Event {
	return []Event{
		withGeo(withRef(view(site, "2026-03-08T10:00:00Z", "/a", "h1"), "news.example.org"), "GB", "London"),
		withGeo(withRef(view(site, "2026-03-08T11:00:00Z", "/a", "h1"), "news.example.org"), "GB", "London"),
		withGeo(withClient(view(site, "2026-03-08T11:30:00Z", "/b", "h2"), "Chrome", "Windows", DeviceMobile), "FR", "Paris"),
		withGeo(withRef(view(site, "2026-03-09T09:00:00Z", "/a", "h3"), "news.example.org"), "GB", "Leeds"),
		withClient(view(site, "2026-03-09T09:30:00Z", "/b", "h4"), "Safari", "iOS", DeviceMobile),
		withClient(view(site, "2026-03-09T09:45:00Z", "/c", ""), "Other", "Other", DeviceBot),
		notFound(view(site, "2026-03-09T10:00:00Z", "/missing", "h3")),
		view(site, "2026-03-10T08:00:00Z", "/a", "h6"),
		view(site, "2026-03-10T08:10:00Z", "/d", "h7"),
		view(site, "2026-03-10T08:20:00Z", "/d", "h8"),
	}
}

func withRef(e Event, host string) Event { e.ReferrerHost = host; return e }

func withGeo(e Event, country, city string) Event { e.Country, e.City = country, city; return e }

func withClient(e Event, browser, os, device string) Event {
	e.Browser, e.OS, e.Device = browser, os, device
	return e
}

func notFound(e Event) Event { e.StatusClass = StatusNotFound; return e }

func checkFixedReport(t *testing.T, report Report) {
	t.Helper()
	if report.Totals != (Totals{Views: 8, Uniques: 7, BotViews: 1}) {
		t.Errorf("totals = %+v", report.Totals)
	}
	wantSeries := []Point{{day("2026-03-08"), 3, 2}, {day("2026-03-09"), 2, 2}, {day("2026-03-10"), 3, 3}}
	if len(report.Series) != len(wantSeries) {
		t.Fatalf("series = %+v", report.Series)
	}
	for i, want := range wantSeries {
		if report.Series[i] != want {
			t.Errorf("series[%d] = %+v, want %+v", i, report.Series[i], want)
		}
	}
	wantPaths := []Ranked{{"/a", 4, 3}, {"/b", 2, 2}, {"/d", 2, 2}}
	if len(report.Paths) != len(wantPaths) {
		t.Fatalf("paths = %+v", report.Paths)
	}
	for i, want := range wantPaths {
		if report.Paths[i] != want {
			t.Errorf("paths[%d] = %+v, want %+v", i, report.Paths[i], want)
		}
	}
	checks := map[string][]Ranked{
		"referrers": {{"news.example.org", 3, 2}},
		"countries": {{"GB", 3, 2}, {"FR", 1, 1}},
		"cities":    {{"London", 2, 1}, {"Leeds", 1, 1}, {"Paris", 1, 1}},
		"statuses":  {{"2xx", 8, 7}, {"4xx", 1, 1}},
	}
	got := map[string][]Ranked{"referrers": report.Referrers, "countries": report.Countries, "cities": report.Cities, "statuses": report.Statuses}
	for name, want := range checks {
		if len(got[name]) != len(want) {
			t.Errorf("%s = %+v, want %+v", name, got[name], want)
			continue
		}
		for i := range want {
			if got[name][i] != want[i] {
				t.Errorf("%s[%d] = %+v, want %+v", name, i, got[name][i], want[i])
			}
		}
	}
	if v := valuesOf(report.Devices); v[DeviceDesktop] != 6 || v[DeviceMobile] != 2 || v[DeviceBot] != 1 {
		t.Errorf("devices = %+v", report.Devices)
	}
	if v := valuesOf(report.Browsers); v["Firefox"] != 6 || v["Chrome"] != 1 || v["Safari"] != 1 {
		t.Errorf("browsers = %+v", report.Browsers)
	}
	if v := valuesOf(report.Systems); v["Linux"] != 6 || v["Windows"] != 1 || v["iOS"] != 1 {
		t.Errorf("systems = %+v", report.Systems)
	}
}

func TestReportFromRawEvents(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		checkFixedReport(t, e.report("demo", rangeParams("2026-03-08", "2026-03-10")))
	})
}

func TestReportZeroFillsTheSeries(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		report := e.report("demo", rangeParams("2026-03-06", "2026-03-10"))
		if len(report.Series) != 5 || report.Series[0].Views != 0 || report.Series[1].Views != 0 || report.Series[2].Views != 3 {
			t.Errorf("series = %+v", report.Series)
		}
	})
}

func TestReportHourlySeries(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		p := rangeParams("2026-03-08", "2026-03-08")
		p.Interval = IntervalHour
		report := e.report("demo", p)
		if len(report.Series) != 24 {
			t.Fatalf("series length = %d, want 24", len(report.Series))
		}
		if report.Series[10] != (Point{at("2026-03-08T10:00:00Z"), 1, 1}) || report.Series[11] != (Point{at("2026-03-08T11:00:00Z"), 2, 2}) {
			t.Errorf("hours 10 and 11 = %+v, %+v", report.Series[10], report.Series[11])
		}
		if report.Totals.Views != 3 || report.Totals.Uniques != 2 {
			t.Errorf("totals = %+v", report.Totals)
		}
	})
}

func TestTopLimitsTheRankedLists(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		p := rangeParams("2026-03-08", "2026-03-10")
		p.Top = 2
		report := e.report("demo", p)
		if len(report.Paths) != 2 || report.Paths[0].Value != "/a" || report.Paths[1].Value != "/b" {
			t.Errorf("paths = %+v", report.Paths)
		}
	})
}

func TestReportOnlyCoversTheRequestedRange(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		report := e.report("demo", rangeParams("2026-03-09", "2026-03-09"))
		if report.Totals.Views != 2 || report.Totals.BotViews != 1 {
			t.Errorf("totals = %+v", report.Totals)
		}
	})
}

func TestReportOfASiteWithoutDataIsEmpty(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.site("quiet")
		report := e.report("quiet", rangeParams("2026-03-08", "2026-03-10"))
		if report.Totals != (Totals{}) || len(report.Series) != 3 {
			t.Errorf("report = %+v", report)
		}
		if report.GeneratedAt.IsZero() || report.LocationAvailable {
			t.Errorf("report = %+v", report)
		}
	})
}

type availability bool

func (a availability) Available() bool { return bool(a) }

func TestReportReportsLocationAvailability(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.site("demo")
		e.svc = New(db, e.sites, e.ws, e.settings, Options{Now: e.clock.Now, Location: availability(true)})
		if !e.report("demo", rangeParams("2026-03-08", "2026-03-10")).LocationAvailable {
			t.Error("locationAvailable is false")
		}
	})
}

func TestInstanceReportShowsDroppedEvents(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		recorder := NewRecorder(db, RecorderOptions{Hasher: e.hasher, Logger: discardLogger(), BufferSize: 1})
		recorder.Record(hitFor(site.ID))
		recorder.Record(hitFor(site.ID))
		e.svc = New(db, e.sites, e.ws, e.settings, Options{Now: e.clock.Now, Recorder: recorder})
		instance, err := e.svc.InstanceReport(context.Background(), rangeParams("2026-03-08", "2026-03-10"))
		if err != nil {
			t.Fatal(err)
		}
		if instance.DroppedEvents == nil || *instance.DroppedEvents != 1 {
			t.Errorf("droppedEvents = %v", instance.DroppedEvents)
		}
		if instance.Totals.Views != 8 {
			t.Errorf("instance totals = %+v", instance.Totals)
		}
		if site := e.report("demo", rangeParams("2026-03-08", "2026-03-10")); site.DroppedEvents != nil {
			t.Error("a site report exposes droppedEvents")
		}
	})
}
