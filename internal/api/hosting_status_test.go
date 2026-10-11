package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func (a actor) site(slug string) siteResponse {
	a.h.t.Helper()
	return decodeBody[siteEnvelope](a.h.t, a.expect(http.StatusOK, "GET", "/api/v1/sites/"+slug, nil)).Site
}

func (a actor) hostingStatus() hostingStatusResponse {
	a.h.t.Helper()
	return decodeBody[hostingStatusResponse](a.h.t, a.expect(http.StatusOK, "GET", "/api/v1/hosting/status", nil))
}

func noticeCodes(notices []noticeResponse) []string {
	codes := make([]string, len(notices))
	for i, n := range notices {
		codes[i] = n.Code
	}
	return codes
}

func requireCodes(t *testing.T, label string, notices []noticeResponse, want ...string) {
	t.Helper()
	requireSlugs(t, label, noticeCodes(notices), want)
}

func (h *harness) enableSubdomainMode(root actor) {
	h.t.Helper()
	root.setSettings(`{"settings":{"hosting.baseDomain":"sites.example.com"}}`)
	if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
		h.t.Fatal(err)
	}
	root.setSettings(`{"settings":{"hosting.subdomainEnabled":true}}`)
}

func reasonOf(m modeResponse) string {
	if m.Reason == nil {
		return ""
	}
	return *m.Reason
}

func TestSiteHostingObjectFollowsEveryState(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")

		empty := root.createSite("acme", createSiteRequest{Slug: "acme-empty"})
		if empty.Hosting.Reachable || empty.Hosting.Modes.Path.State != "on" {
			t.Errorf("site without a version = %+v", empty.Hosting)
		}
		requireCodes(t, "empty notices", empty.Hosting.Notices, "no_live_version", "subdomain_off")
		root.publishLive("acme-docs", tinySite)

		docs := root.site("acme-docs")
		if !docs.Hosting.Reachable || docs.Hosting.URL == nil || *docs.Hosting.URL != "https://example.com/s/acme-docs/" {
			t.Fatalf("live site = %+v", docs.Hosting)
		}
		if docs.Hosting.Modes.Path.State != "on" || docs.Hosting.Modes.Path.Reason != nil {
			t.Errorf("path mode = %+v", docs.Hosting.Modes.Path)
		}
		if docs.Hosting.Modes.Subdomain.State != "off" || reasonOf(docs.Hosting.Modes.Subdomain) != "switched_off" {
			t.Errorf("subdomain mode = %+v", docs.Hosting.Modes.Subdomain)
		}
		requireCodes(t, "docs notices", docs.Hosting.Notices, "subdomain_off")
		if got := docs.Hosting.Notices[0].Message; got != "The subdomain address for this site is switched off." {
			t.Errorf("message = %q", got)
		}

		h.enableSubdomainMode(root)
		docs = root.site("acme-docs")
		if docs.Hosting.Modes.Subdomain.State != "on" || len(docs.Hosting.Notices) != 0 {
			t.Errorf("both modes on = %+v", docs.Hosting)
		}

		if _, err := h.settings.RecordWildcard(context.Background(), false, "no answer"); err != nil {
			t.Fatal(err)
		}
		docs = root.site("acme-docs")
		if docs.Hosting.Modes.Subdomain.State != "on" || reasonOf(docs.Hosting.Modes.Subdomain) != "wildcard_check_failed" {
			t.Errorf("degraded subdomain mode = %+v", docs.Hosting.Modes.Subdomain)
		}
		requireCodes(t, "degraded notices", docs.Hosting.Notices, "subdomain_unreliable")
		for _, notice := range docs.Hosting.Notices {
			if strings.Contains(notice.Message, "no answer") {
				t.Errorf("notice leaks admin detail: %q", notice.Message)
			}
		}

		root.setSettings(`{"settings":{"hosting.pathEnabled":false}}`)
		docs = root.site("acme-docs")
		if !docs.Hosting.Reachable || docs.Hosting.Modes.Path.State != "off" || reasonOf(docs.Hosting.Modes.Path) != "switched_off" {
			t.Errorf("only subdomain mode on = %+v", docs.Hosting)
		}
		listed := decodeBody[siteListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/sites", nil)).Sites
		for _, s := range listed {
			if s.Slug == "acme-docs" && s.Hosting.Modes.Path.State != "off" {
				t.Errorf("listed hosting = %+v", s.Hosting)
			}
		}
	})
}

func TestGatedSiteNeedsHTTPSInPathMode(t *testing.T) {
	cases := []struct {
		name      string
		opts      harnessOptions
		reachable bool
		reason    string
	}{
		{"plain http", harnessOptions{}, false, "https_required"},
		{"https", harnessOptions{public: httpsPublic}, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storetest.Each(t, func(t *testing.T, db store.Store) {
				h := newHarness(t, db, tc.opts)
				s := newAccessSetup(t, h)
				s.alex.setAccess(map[string]any{"mode": "restricted", "password": siteSecret})

				site := s.alex.site("guarded")
				if site.Hosting.Reachable != tc.reachable || reasonOf(site.Hosting.Modes.Path) != tc.reason {
					t.Errorf("hosting = %+v", site.Hosting)
				}
				if !tc.reachable {
					requireCodes(t, "notices", site.Hosting.Notices, "subdomain_off", "path_https_required", "unreachable")
				}
			})
		})
	}
}

func TestPublishResponsesCarryTheHostingObject(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")

		pending := root.publishNewSite("acme-docs", tinySite)
		if pending.Hosting.Reachable || pending.Hosting.Modes.Path.State != "on" {
			t.Errorf("pending hosting = %+v", pending.Hosting)
		}
		requireCodes(t, "pending notices", pending.Hosting.Notices, "no_live_version", "subdomain_off")

		root.uploadFiles(pending, tinySite)
		finalized := decodeBody[resultResponse](t, root.finalize(pending.Slug, pending.VersionID))
		if !finalized.Hosting.Reachable || finalized.Hosting.URL == nil {
			t.Errorf("finalized hosting = %+v", finalized.Hosting)
		}
		requireCodes(t, "finalized notices", finalized.Hosting.Notices, "subdomain_off")

		raw := root.expect(http.StatusOK, "POST", "/api/v1/publish/acme-docs/versions/"+pending.VersionID+"/restore", nil).Body.Bytes()
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if _, ok := fields["hosting"]; !ok {
			t.Errorf("restore response has no hosting object: %s", raw)
		}
	})
}

func TestHostingStatusNeedsAuthentication(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.rootActor()
		expectError(t, h.do("GET", "/api/v1/hosting/status", nil), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestHostingStatusDetailIsRootOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")

		healthy := root.hostingStatus()
		if healthy.Degraded || healthy.Detail == nil || healthy.Detail.WildcardStatus != "unchecked" {
			t.Errorf("healthy root status = %+v", healthy)
		}
		requireCodes(t, "healthy notices", healthy.Notices, "subdomain_off")

		h.enableSubdomainMode(root)
		if _, err := h.settings.RecordWildcard(context.Background(), false, "resolver timed out"); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if _, err := h.settings.RecordScheduledWildcard(context.Background(), false, "resolver timed out"); err != nil {
				t.Fatal(err)
			}
		}
		degraded := root.hostingStatus()
		if !degraded.Degraded || degraded.Modes.Subdomain.State != "on" || reasonOf(degraded.Modes.Subdomain) != "wildcard_check_failed" {
			t.Fatalf("degraded status = %+v", degraded)
		}
		detail := degraded.Detail
		if detail == nil || detail.WildcardStatus != "failed" || detail.WildcardDetail != "resolver timed out" ||
			detail.ConsecutiveFailures != 2 || detail.WildcardCheckedAt == nil || detail.SubdomainDisabledReason != nil {
			t.Errorf("root detail = %+v", detail)
		}

		if _, err := h.settings.RecordScheduledWildcard(context.Background(), false, "resolver timed out"); err != nil {
			t.Fatal(err)
		}
		off := root.hostingStatus()
		if off.Modes.Subdomain.State != "off" || reasonOf(off.Modes.Subdomain) != "wildcard_check_failed" {
			t.Errorf("auto-disabled status = %+v", off)
		}
		if off.Detail == nil || off.Detail.SubdomainDisabledReason == nil || *off.Detail.SubdomainDisabledReason != "wildcard_check_failed" {
			t.Errorf("auto-disabled detail = %+v", off.Detail)
		}

		rec := sam.expect(http.StatusOK, "GET", "/api/v1/hosting/status", nil)
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["detail"]; ok {
			t.Errorf("non-root status has a detail key: %s", rec.Body.String())
		}
		for _, leaked := range []string{"resolver timed out", "wildcardDetail", "consecutiveFailures"} {
			if strings.Contains(rec.Body.String(), leaked) {
				t.Errorf("non-root status leaks %q: %s", leaked, rec.Body.String())
			}
		}
		member := decodeBody[hostingStatusResponse](t, rec)
		requireCodes(t, "non-root notices", member.Notices, "subdomain_off")
	})
}

func TestNonRootUsersSeeNoticesOnlyForSitesTheyCanRead(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.createWorkspace("Other Co", "other")
		sam := h.bearerActor("sam@example.com")
		root.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read"})
		root.publishLive("acme-docs", tinySite)
		root.createSite("acme", createSiteRequest{Slug: "acme-draft"})
		root.createSite("other", createSiteRequest{Slug: "other-site"})

		docs := sam.site("acme-docs")
		if !docs.Hosting.Reachable {
			t.Errorf("member view = %+v", docs.Hosting)
		}
		requireCodes(t, "member notices", docs.Hosting.Notices, "subdomain_off")
		draft := sam.site("acme-draft")
		requireCodes(t, "draft notices", draft.Hosting.Notices, "no_live_version", "subdomain_off")

		listed := sam.siteSlugs("")
		requireSlugs(t, "visible sites", listed, []string{"acme-docs", "acme-draft"})
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/private-site", nil)

		status := sam.expect(http.StatusOK, "GET", "/api/v1/hosting/status", nil).Body.String()
		for _, hidden := range []string{"other-site", "other", "Other Co", "acme-docs", "acme-draft"} {
			if strings.Contains(status, hidden) {
				t.Errorf("hosting status mentions %q: %s", hidden, status)
			}
		}
	})
}
