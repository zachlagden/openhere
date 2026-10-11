package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

const customName = "www.example.org"

func TestDomainLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		added := s.alex.addDomain("guarded", "WWW.Example.org.")
		if added.Domain != customName || added.Status != "pending_verification" || added.IsPrimary || added.VerifiedAt != nil || added.LastCheckedAt != nil || added.LastCheckResult != nil {
			t.Fatalf("added = %+v", added)
		}
		if added.Challenge.RecordType != "TXT" || added.Challenge.Name != "_selfhost-challenge."+customName || len(added.Challenge.Value) < 32 {
			t.Errorf("challenge = %+v", added.Challenge)
		}
		if added.Target.RecordType != "CNAME" || added.Target.Value != testHost || !strings.Contains(added.Target.Note, "A and AAAA") {
			t.Errorf("target = %+v", added.Target)
		}
		list := decodeBody[domainListResponse](t, s.alex.expect(http.StatusOK, "GET", domainsPath("guarded"), nil))
		if len(list.Domains) != 1 || list.Domains[0].ID != added.ID {
			t.Errorf("list = %+v", list)
		}
		got := decodeBody[domainEnvelope](t, s.alex.expect(http.StatusOK, "GET", domainsPath("guarded")+"/"+added.ID, nil)).Domain
		if got.ID != added.ID || got.Challenge != added.Challenge {
			t.Errorf("get = %+v", got)
		}

		missing := s.alex.verifyDomain("guarded", added.ID)
		if missing.Verified || missing.Domain.Status != "pending_verification" || missing.Domain.LastCheckResult == nil || *missing.Domain.LastCheckResult != "record_not_found" ||
			!strings.Contains(missing.Message, "_selfhost-challenge.www.example.org") {
			t.Errorf("verify without a record = %+v", missing)
		}
		s.h.dns.publish(added.Challenge.Name, "wrong")
		if mismatch := s.alex.verifyDomain("guarded", added.ID); mismatch.Verified || !strings.Contains(mismatch.Message, "none holds the value shown") {
			t.Errorf("verify with a wrong value = %+v", mismatch)
		}
		s.h.dns.fail(true)
		if flaky := s.alex.verifyDomain("guarded", added.ID); flaky.Verified || !strings.Contains(flaky.Message, "usually temporary") {
			t.Errorf("verify during a DNS failure = %+v", flaky)
		}
		s.h.dns.fail(false)
		s.h.dns.publish(added.Challenge.Name, "other", added.Challenge.Value)
		done := s.alex.verifyDomain("guarded", added.ID)
		if !done.Verified || done.Domain.Status != "verified" || done.Domain.VerifiedAt == nil || !strings.Contains(done.Message, "matches") {
			t.Errorf("verify = %+v", done)
		}

		s.alex.expect(http.StatusNoContent, "DELETE", domainsPath("guarded")+"/"+added.ID, nil)
		s.alex.expectError(http.StatusNotFound, "not_found", "GET", domainsPath("guarded")+"/"+added.ID, nil)
		s.alex.expectError(http.StatusNotFound, "not_found", "DELETE", domainsPath("guarded")+"/"+added.ID, nil)
	})
}

func TestDomainValidationAndConflicts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.publishLive("other", siteFiles{"index.html": "other"})
		for _, bad := range []string{"", "localhost", "192.0.2.1", "*.example.org", "example.org:8080", "https://example.org", "intranet.local", testHost, "demo." + testHost, "a..example.org"} {
			s.alex.expectError(http.StatusBadRequest, "validation_failed", "POST", domainsPath("guarded"), domainCreateRequest{Domain: bad})
		}
		s.alex.addDomain("guarded", customName)
		s.alex.expectError(http.StatusConflict, "domain_taken", "POST", domainsPath("guarded"), domainCreateRequest{Domain: customName})
		s.alex.addDomain("other", "WWW.example.org")
		s.alex.verifiedDomain("guarded", "taken.example.org")
		s.alex.expectError(http.StatusConflict, "domain_taken", "POST", domainsPath("other"), domainCreateRequest{Domain: "Taken.example.org"})
		s.alex.expectError(http.StatusBadRequest, "invalid_json", "POST", domainsPath("guarded"), `{"domain": "a.example.org", "extra": 1}`)
	})
}

func TestDomainLimitPerSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", settingsPatch("hosting.maxDomainsPerSite", 2))
		s.alex.addDomain("guarded", "a.example.org")
		s.alex.addDomain("guarded", "b.example.org")
		s.alex.expectError(http.StatusConflict, "domain_limit_reached", "POST", domainsPath("guarded"), domainCreateRequest{Domain: "c.example.org"})
	})
}

func TestDomainPermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		d := s.alex.verifiedDomain("guarded", customName)
		path := domainsPath("guarded")
		one := path + "/" + d.ID
		type call struct {
			method, path string
			body         any
		}
		reads := []call{{"GET", path, nil}, {"GET", one, nil}}
		writes := []call{
			{"POST", path, domainCreateRequest{Domain: "new.example.org"}},
			{"POST", one + "/verify", nil},
			{"PATCH", one, map[string]any{"isPrimary": true}},
			{"DELETE", one, nil},
		}
		for _, c := range reads {
			s.sam.expect(http.StatusOK, c.method, c.path, c.body)
			s.kim.expect(http.StatusOK, c.method, c.path, c.body)
			s.lee.expectError(http.StatusNotFound, "not_found", c.method, c.path, c.body)
			expectError(t, s.h.do(c.method, c.path, c.body), http.StatusUnauthorized, "unauthenticated")
		}
		for _, c := range writes {
			s.sam.expectError(http.StatusForbidden, "forbidden", c.method, c.path, c.body)
			s.lee.expectError(http.StatusNotFound, "not_found", c.method, c.path, c.body)
			expectError(t, s.h.do(c.method, c.path, c.body), http.StatusUnauthorized, "unauthenticated")
		}
		s.kim.expect(http.StatusCreated, "POST", path, domainCreateRequest{Domain: "new.example.org"})
		s.kim.expect(http.StatusOK, "POST", one+"/verify", nil)
		s.kim.expect(http.StatusOK, "PATCH", one, map[string]any{"isPrimary": true})
		s.kim.expect(http.StatusNoContent, "DELETE", one, nil)
		s.alex.expectError(http.StatusNotFound, "not_found", "GET", domainsPath("nothing")+"/"+d.ID, nil)
	})
}

func TestDomainWritesNeedTheCSRFHeaderForCookies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		session := s.h.sessionFor("kim@example.com")
		bare := s.h.do("POST", domainsPath("guarded"), domainCreateRequest{Domain: "a.example.org"}, withCookie(session))
		expectError(t, bare, http.StatusForbidden, "csrf_rejected")
		ok := s.h.do("POST", domainsPath("guarded"), domainCreateRequest{Domain: "a.example.org"}, withCookie(session), browser())
		expectStatus(t, ok, http.StatusCreated)
	})
}

func TestPrimaryDomainRules(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		pending := s.alex.addDomain("guarded", "pending.example.org")
		first := s.alex.verifiedDomain("guarded", "first.example.org")
		second := s.alex.verifiedDomain("guarded", "second.example.org")
		s.alex.expectError(http.StatusConflict, "domain_not_verified", "PATCH", domainsPath("guarded")+"/"+pending.ID, map[string]any{"isPrimary": true})
		s.alex.expectError(http.StatusBadRequest, "validation_failed", "PATCH", domainsPath("guarded")+"/"+first.ID, map[string]any{})

		if !s.alex.setPrimary(first.ID, true).IsPrimary {
			t.Error("first is not primary")
		}
		s.alex.setPrimary(second.ID, true)
		list := decodeBody[domainListResponse](t, s.alex.expect(http.StatusOK, "GET", domainsPath("guarded"), nil)).Domains
		primaries := map[string]bool{}
		for _, d := range list {
			primaries[d.Domain] = d.IsPrimary
		}
		if primaries["first.example.org"] || !primaries["second.example.org"] {
			t.Errorf("primaries = %v", primaries)
		}
		if s.alex.setPrimary(second.ID, false).IsPrimary {
			t.Error("primary was not cleared")
		}
	})
}

func TestSiteURLAndHostingFollowCustomDomains(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		base := s.alex.guardedSite()
		if base.URL == nil || *base.URL != "http://example.com/s/guarded/" {
			t.Fatalf("base url = %v", base.URL)
		}
		pending := s.alex.addDomain("guarded", "pending.example.org")
		waiting := s.alex.guardedSite()
		requireCodes(t, "pending notices", waiting.Hosting.Notices, "subdomain_off", "custom_domain_pending")
		if *waiting.URL != *base.URL || !waiting.Hosting.Reachable || waiting.Hosting.Modes.CustomDomains.State != "on" {
			t.Errorf("hosting with a pending domain = %+v", waiting.Hosting)
		}
		if got := waiting.Hosting.Notices[1].Message; got != "A custom domain is waiting for DNS verification." {
			t.Errorf("message = %q", got)
		}

		verified := s.alex.verifiedDomain("guarded", "www.example.org")
		unpicked := s.alex.guardedSite()
		if *unpicked.URL != *base.URL {
			t.Errorf("url without a primary = %q", *unpicked.URL)
		}
		s.alex.setPrimary(verified.ID, true)
		primary := s.alex.guardedSite()
		if *primary.URL != "http://www.example.org/" || *primary.Hosting.URL != "http://www.example.org/" {
			t.Errorf("url with a primary = %q", *primary.URL)
		}
		listed := s.alex.siteSlugs("")
		if len(listed) != 1 {
			t.Fatalf("slugs = %v", listed)
		}

		s.alex.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", settingsPatch("hosting.customDomainsEnabled", false))
		off := s.alex.guardedSite()
		if *off.URL != *base.URL || off.Hosting.Modes.CustomDomains.State != "off" {
			t.Errorf("hosting with the feature off = %+v", off.Hosting)
		}
		requireCodes(t, "off notices", off.Hosting.Notices, "subdomain_off", "custom_domains_off")
		status := decodeBody[hostingStatusResponse](t, s.sam.expect(http.StatusOK, "GET", "/api/v1/hosting/status", nil))
		if status.Modes.CustomDomains.State != "off" {
			t.Errorf("instance status = %+v", status.Modes)
		}
		s.alex.expect(http.StatusNoContent, "DELETE", domainsPath("guarded")+"/"+pending.ID, nil)
	})
}

func TestVerifyIsRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := limitsWith(func(l *Limits) { l.DomainVerify = Rule{Max: 2, Window: time.Hour} })
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{limits: limits}))
		d := s.alex.addDomain("guarded", customName)
		path := domainsPath("guarded") + "/" + d.ID + "/verify"
		s.alex.expect(http.StatusOK, "POST", path, nil)
		s.alex.expect(http.StatusOK, "POST", path, nil)
		rec := s.alex.call("POST", path, nil)
		expectError(t, rec, http.StatusTooManyRequests, "rate_limited")
		if rec.Header().Get("Retry-After") == "" {
			t.Error("no Retry-After header")
		}
	})
}

func TestDomainSettingsAreRootOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		for _, body := range []map[string]any{settingsPatch("hosting.customDomainsEnabled", false), settingsPatch("hosting.maxDomainsPerSite", 3)} {
			s.kim.expectError(http.StatusNotFound, "not_found", "PATCH", "/api/v1/admin/settings", body)
			s.alex.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", body)
		}
		for _, bad := range []any{0, 51, "ten", 2.5} {
			s.alex.expectError(http.StatusBadRequest, "validation_failed", "PATCH", "/api/v1/admin/settings", settingsPatch("hosting.maxDomainsPerSite", bad))
		}
		s.alex.expectError(http.StatusBadRequest, "validation_failed", "PATCH", "/api/v1/admin/settings", settingsPatch("hosting.customDomainsEnabled", "yes"))
	})
}

func TestBaseDomainCannotMoveOverACustomDomain(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.addDomain("guarded", "www.example.org")
		s.alex.expectError(http.StatusConflict, "base_domain_in_use", "PATCH", "/api/v1/admin/settings", settingsPatch("hosting.baseDomain", "example.org"))
	})
}

func settingsPatch(key string, value any) map[string]any {
	return map[string]any{"settings": map[string]any{key: value}}
}

func TestAVerifyingSiteDisplacesAnUnverifiedClaimOnTheSameName(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.publishLive("other", siteFiles{"index.html": "other"})
		squatted := s.alex.addDomain("other", customName)
		owner := s.alex.addDomain("guarded", customName)
		if squatted.Status != owner.Status {
			t.Errorf("claims differ in status: %q and %q", squatted.Status, owner.Status)
		}
		s.h.dns.publish(owner.Challenge.Name, owner.Challenge.Value)
		if result := s.alex.verifyDomain("guarded", owner.ID); !result.Verified {
			t.Fatalf("owner did not verify: %+v", result)
		}
		s.alex.expectError(http.StatusNotFound, "not_found", "GET", domainsPath("other")+"/"+squatted.ID, nil)
		s.alex.expectError(http.StatusConflict, "domain_taken", "POST", domainsPath("other"), domainCreateRequest{Domain: customName})
	})
}
