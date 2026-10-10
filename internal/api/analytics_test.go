package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const (
	firefoxUA = "Mozilla/5.0 (X11; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0"
	chromeUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	botUA     = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
)

var analyticsSite = siteFiles{"index.html": "<h1>home</h1>", "about.html": "<h1>about</h1>", "app.js": "console.log(1)"}

func (h *harness) visit(target, userAgent, referrer, remote string) {
	h.t.Helper()
	req := httptest.NewRequestWithContext(h.t.Context(), http.MethodGet, "http://"+testHost+target, nil)
	req.Header.Set("User-Agent", userAgent)
	if referrer != "" {
		req.Header.Set("Referer", referrer)
	}
	req.RemoteAddr = remote
	h.routed.ServeHTTP(httptest.NewRecorder(), req)
}

func (h *harness) flush() {
	h.t.Helper()
	if err := h.recorder.Flush(context.Background()); err != nil {
		h.t.Fatalf("flush: %v", err)
	}
}

func decodeAnalytics(t *testing.T, rec *httptest.ResponseRecorder) analyticsResponse {
	t.Helper()
	expectStatus(t, rec, http.StatusOK)
	return decodeBody[analyticsResponse](t, rec)
}

func rankedMap(list []analyticsRankedResponse) map[string]int64 {
	out := map[string]int64{}
	for _, item := range list {
		out[item.Value] = item.Views
	}
	return out
}

func seedVisits(h *harness) {
	h.visit("/s/guide/", firefoxUA, "https://news.example.org/story?id=1", "192.0.2.10:1000")
	h.visit("/s/guide/", firefoxUA, "https://news.example.org/story?id=2", "192.0.2.10:1001")
	h.visit("/s/guide/about?token=secret-token-123", chromeUA, "", "192.0.2.20:1000")
	h.visit("/s/guide/app.js", chromeUA, "", "192.0.2.20:1000")
	h.visit("/s/guide/", botUA, "", "192.0.2.30:1000")
	h.flush()
}

func TestSiteAnalyticsEndToEnd(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.publishLive("guide", analyticsSite)
		seedVisits(h)

		rec := root.call("GET", "/api/v1/sites/guide/analytics", nil)
		report := decodeAnalytics(t, rec)
		if report.Totals.Views != 3 || report.Totals.Uniques != 2 || report.Totals.BotViews != 1 {
			t.Errorf("totals = %+v", report.Totals)
		}
		if got := rankedMap(report.Paths); got["/"] != 2 || got["/about"] != 1 || len(got) != 2 {
			t.Errorf("paths = %v", got)
		}
		if got := rankedMap(report.Referrers); got["news.example.org"] != 2 || len(got) != 1 {
			t.Errorf("referrers = %v", got)
		}
		if got := rankedMap(report.Browsers); got["Firefox"] != 2 || got["Chrome"] != 1 {
			t.Errorf("browsers = %v", got)
		}
		if got := rankedMap(report.OperatingSystems); got["Linux"] != 2 || got["Windows"] != 1 {
			t.Errorf("operating systems = %v", got)
		}
		if got := rankedMap(report.Devices); got["desktop"] != 3 || got["bot"] != 1 {
			t.Errorf("devices = %v", got)
		}
		if got := rankedMap(report.Statuses); got["2xx"] != 3 {
			t.Errorf("statuses = %v", got)
		}
		if len(report.Series) != 30 || report.Interval != "day" || report.LocationAvailable || report.DroppedEvents != nil {
			t.Errorf("series = %d, interval = %q, location = %v, dropped = %v", len(report.Series), report.Interval, report.LocationAvailable, report.DroppedEvents)
		}
		last := report.Series[len(report.Series)-1]
		if last.Views != 3 || last.Uniques != 2 {
			t.Errorf("last point = %+v", last)
		}
		if report.GeneratedAt.IsZero() || report.From == "" || report.To == "" {
			t.Errorf("report = %+v", report)
		}
		if strings.Contains(rec.Body.String(), "droppedEvents") {
			t.Error("a site report exposes droppedEvents")
		}
		if report.Countries == nil || report.Cities == nil {
			t.Error("empty lists must be arrays, not null")
		}
	})
}

func TestAnalyticsStoresNoIPNoUserAgentNoQuery(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.publishLive("guide", analyticsSite)
		seedVisits(h)
		var dump strings.Builder
		for _, query := range []string{
			"SELECT site_id, path, referrer_host, country, city, browser, os, device, COALESCE(visitor, '') FROM analytics_events",
			"SELECT key, value FROM instance_settings WHERE key LIKE 'internal.%'",
		} {
			rows, err := db.QueryContext(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			cols, _ := rows.Columns()
			for rows.Next() {
				values := make([]string, len(cols))
				ptrs := make([]any, len(cols))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					t.Fatal(err)
				}
				dump.WriteString(strings.Join(values, "|") + "\n")
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
		}
		for _, forbidden := range []string{"192.0.2.", "Gecko", "Mozilla", "Googlebot", "token=", "secret-token-123", "id=1", "/story"} {
			if strings.Contains(dump.String(), forbidden) {
				t.Errorf("the database contains %q", forbidden)
			}
		}
		if !strings.Contains(dump.String(), "news.example.org") {
			t.Error("the referrer host is missing")
		}
	})
}

func TestTurningAnalyticsOffRecordsNothingAndKeepsOldData(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.publishLive("guide", analyticsSite)
		seedVisits(h)
		root.expect(http.StatusOK, "PATCH", "/api/v1/sites/guide", `{"analyticsEnabled":false}`)
		h.visit("/s/guide/", chromeUA, "", "192.0.2.50:1000")
		h.visit("/s/guide/about", chromeUA, "", "192.0.2.51:1000")
		h.flush()
		report := decodeAnalytics(t, root.call("GET", "/api/v1/sites/guide/analytics", nil))
		if report.Totals.Views != 3 {
			t.Errorf("views = %d, want the 3 recorded before the switch", report.Totals.Views)
		}
		root.expect(http.StatusOK, "PATCH", "/api/v1/sites/guide", `{"analyticsEnabled":true}`)
		h.visit("/s/guide/", chromeUA, "", "192.0.2.52:1000")
		h.flush()
		if got := decodeAnalytics(t, root.call("GET", "/api/v1/sites/guide/analytics", nil)).Totals.Views; got != 4 {
			t.Errorf("views = %d, want 4 after switching back on", got)
		}
	})
}

func TestAnalyticsPermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		site := root.createSite("acme", createSiteRequest{Slug: "guide"})
		root.createSite("acme", createSiteRequest{Slug: "blog"})
		sam := h.bearerActor("sam@example.com")
		kim := h.bearerActor("kim@example.com")

		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/guide/analytics", nil)
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/nope/analytics", nil)
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/workspaces/acme/analytics", nil)

		root.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read", ScopeType: "site", ScopeID: site.ID})
		sam.expect(http.StatusOK, "GET", "/api/v1/sites/guide/analytics", nil)
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/blog/analytics", nil)
		sam.expectError(http.StatusForbidden, codeForbidden, "GET", "/api/v1/workspaces/acme/analytics", nil)

		root.addMember("acme", addMemberRequest{Email: "kim@example.com", Role: "read", ScopeType: "workspace"})
		kim.expect(http.StatusOK, "GET", "/api/v1/workspaces/acme/analytics", nil)
		kim.expect(http.StatusOK, "GET", "/api/v1/sites/blog/analytics", nil)

		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/admin/analytics", nil)
		kim.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/admin/analytics", nil)
	})
}

func TestWorkspaceAnalyticsAggregatesItsSitesOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.createWorkspace("Globex Ltd", "globex")
		root.publishLive("guide", analyticsSite)
		root.publishLive("blog", analyticsSite)
		seedVisits(h)
		h.visit("/s/blog/", chromeUA, "", "192.0.2.60:1000")
		root.createSite("globex", createSiteRequest{Slug: "elsewhere"})
		h.flush()

		report := decodeAnalytics(t, root.call("GET", "/api/v1/workspaces/acme/analytics", nil))
		if report.Totals.Views != 4 || report.Totals.BotViews != 1 {
			t.Errorf("workspace totals = %+v", report.Totals)
		}
		instance := decodeAnalytics(t, root.call("GET", "/api/v1/admin/analytics", nil))
		if instance.Totals.Views != 4 || instance.DroppedEvents == nil || *instance.DroppedEvents != 0 {
			t.Errorf("instance totals = %+v, dropped = %v", instance.Totals, instance.DroppedEvents)
		}
	})
}

func TestPersonalWorkspaceAnalytics(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		list := decodeBody[workspaceListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/workspaces", nil))
		personal := ""
		for _, ws := range list.Workspaces {
			if ws.Kind == "personal" {
				personal = ws.Slug
			}
		}
		if personal == "" {
			t.Fatal("no personal workspace")
		}
		report := decodeAnalytics(t, root.call("GET", "/api/v1/workspaces/"+personal+"/analytics", nil))
		if report.Totals.Views != 0 || len(report.Series) != 30 {
			t.Errorf("report = %+v", report)
		}
	})
}

func TestAnalyticsQueryParameters(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.publishLive("guide", analyticsSite)
		seedVisits(h)

		hourly := decodeAnalytics(t, root.call("GET", "/api/v1/sites/guide/analytics?from=2026-03-01&to=2026-03-01&interval=hour", nil))
		if hourly.Interval != "hour" || len(hourly.Series) != 24 || hourly.Series[12].Views != 3 {
			t.Errorf("hourly = %d points, hour 12 = %+v", len(hourly.Series), hourly.Series[12])
		}
		top := decodeAnalytics(t, root.call("GET", "/api/v1/sites/guide/analytics?top=1", nil))
		if len(top.Paths) != 1 || top.Paths[0].Value != "/" {
			t.Errorf("top paths = %+v", top.Paths)
		}
		ranged := decodeAnalytics(t, root.call("GET", "/api/v1/sites/guide/analytics?from=2026-02-01T00:00:00Z&to=2026-02-28T00:00:00Z", nil))
		if ranged.Totals.Views != 0 || len(ranged.Series) != 28 {
			t.Errorf("ranged = %+v", ranged.Totals)
		}
		for _, bad := range []string{"from=bad", "interval=week", "top=500", "from=2026-03-05&to=2026-03-01", "from=2020-01-01", "from=2026-01-01&to=2026-03-01&interval=hour"} {
			root.expectError(http.StatusBadRequest, codeValidationFailed, "GET", "/api/v1/sites/guide/analytics?"+bad, nil)
		}
	})
}

func TestAnalyticsNeedsAuthentication(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()
		for _, path := range []string{"/api/v1/sites/guide/analytics", "/api/v1/workspaces/acme/analytics", "/api/v1/admin/analytics"} {
			expectError(t, h.do("GET", path, nil), http.StatusUnauthorized, codeUnauthenticated)
		}
	})
}
