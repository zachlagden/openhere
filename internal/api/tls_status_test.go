package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/certs"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

type fakeTLS struct {
	status certs.Status
	err    error
}

func (f *fakeTLS) Status(context.Context) (certs.Status, error) {
	return f.status, f.err
}

func TestTLSDetailIsReportedToRootOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		expiry := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		soonest := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		reporter := &fakeTLS{status: certs.Status{
			Enabled: true, ManagementExpiresAt: &expiry, DNSProviderConfigured: true,
			Wildcard: certs.WildcardStatus{State: certs.WildcardFailed, Reason: "The wildcard certificate could not be issued."},
			Custom:   certs.CustomSummary{Count: 3, SoonestExpiry: &soonest},
		}}
		h := newHarness(t, db, harnessOptions{tls: reporter})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")

		detail := root.hostingStatus().Detail
		if detail == nil {
			t.Fatal("root has no detail")
		}
		tls := detail.TLS
		if !tls.Enabled || tls.ManagementCertificateExpiresAt == nil || !tls.ManagementCertificateExpiresAt.Equal(expiry) ||
			tls.Wildcard.State != "failed" || tls.Wildcard.Reason == nil || tls.CustomDomains.Count != 3 ||
			tls.CustomDomains.SoonestExpiry == nil || !tls.CustomDomains.SoonestExpiry.Equal(soonest) || !tls.DNSProviderConfigured {
			t.Errorf("tls detail = %+v", tls)
		}

		rec := sam.expect(http.StatusOK, "GET", "/api/v1/hosting/status", nil)
		for _, leaked := range []string{"tls", "managementCertificate", "could not be issued", "dnsProviderConfigured"} {
			if strings.Contains(rec.Body.String(), leaked) {
				t.Errorf("non-root status leaks %q: %s", leaked, rec.Body.String())
			}
		}
	})
}

func TestTLSDetailWhenBuiltInTLSIsOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		tls := root.hostingStatus().Detail.TLS
		if tls.Enabled || tls.Wildcard.State != "none" || tls.CustomDomains.Count != 0 || tls.ManagementCertificateExpiresAt != nil || tls.DNSProviderConfigured {
			t.Errorf("tls detail = %+v", tls)
		}
		root.setSettings(`{"settings":{"tls.dnsProvider":"cloudflare","tls.cloudflareApiToken":"cf-example-token"}}`)
		if !root.hostingStatus().Detail.TLS.DNSProviderConfigured {
			t.Error("the configured provider is not reported")
		}
		for _, path := range []string{"/api/v1/hosting/status", "/api/v1/admin/settings"} {
			if body := root.expect(http.StatusOK, "GET", path, nil).Body.String(); strings.Contains(body, "cf-example-token") {
				t.Errorf("the token leaked from %s: %s", path, body)
			}
		}
		settingsBody := root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil).Body.String()
		if !strings.Contains(settingsBody, `"tls.cloudflareApiTokenSet":true`) || !strings.Contains(settingsBody, `"tls.dnsProvider":"cloudflare"`) {
			t.Errorf("settings = %s", settingsBody)
		}
		sam := h.bearerActor("sam@example.com")
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/admin/settings", nil)
	})
}

func TestFailingWildcardTLSGivesEveryoneAPlainNotice(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		reporter := &fakeTLS{status: certs.Status{Enabled: true, Wildcard: certs.WildcardStatus{State: certs.WildcardFailed, Reason: "secret detail"}}}
		h := newHarness(t, db, harnessOptions{tls: reporter, public: httpsPublic})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")

		requireCodes(t, "subdomain mode off", sam.hostingStatus().Notices, "subdomain_off")

		h.enableSubdomainMode(root)
		for name, who := range map[string]actor{"root": root, "member": sam} {
			notices := who.hostingStatus().Notices
			requireCodes(t, name+" notices", notices, "tls_wildcard_unavailable")
			if notices[0].Message != "Subdomain addresses may show certificate warnings." {
				t.Errorf("%s message = %q", name, notices[0].Message)
			}
		}

		reporter.status.Wildcard = certs.WildcardStatus{State: certs.WildcardOK}
		requireCodes(t, "healthy wildcard", sam.hostingStatus().Notices)
		reporter.status.Wildcard = certs.WildcardStatus{State: certs.WildcardNone}
		requireCodes(t, "no wildcard", sam.hostingStatus().Notices, "tls_wildcard_unavailable")
		reporter.status.Enabled = false
		requireCodes(t, "tls off", sam.hostingStatus().Notices)
	})
}

func TestTLSStatusFailureIsAnInternalError(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{tls: &fakeTLS{err: errors.New("storage unavailable")}})
		root := h.rootActor()
		root.expectError(http.StatusInternalServerError, codeInternal, "GET", "/api/v1/hosting/status", nil)
	})
}
