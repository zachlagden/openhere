package api

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

type fakeDNS struct {
	mu      sync.Mutex
	answers map[string][]string
	failing bool
}

func (f *fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing {
		return nil, &net.DNSError{Err: "server misbehaving", Name: name, IsTemporary: true}
	}
	records, ok := f.answers[name]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	return records, nil
}

func (f *fakeDNS) publish(name string, records ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers[name] = records
}

func (f *fakeDNS) fail(failing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = failing
}

func domainsPath(slug string) string {
	return "/api/v1/sites/" + slug + "/domains"
}

func (a actor) addDomain(slug, name string) domainResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusCreated, "POST", domainsPath(slug), domainCreateRequest{Domain: name})
	return decodeBody[domainEnvelope](a.h.t, rec).Domain
}

func (a actor) verifyDomain(slug, id string) domainVerifyResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusOK, "POST", domainsPath(slug)+"/"+id+"/verify", nil)
	return decodeBody[domainVerifyResponse](a.h.t, rec)
}

func (a actor) verifiedDomain(slug, name string) domainResponse {
	a.h.t.Helper()
	d := a.addDomain(slug, name)
	a.h.dns.publish(d.Challenge.Name, d.Challenge.Value)
	result := a.verifyDomain(slug, d.ID)
	if !result.Verified {
		a.h.t.Fatalf("domain %s did not verify: %+v", name, result)
	}
	return result.Domain
}

func (a actor) setPrimary(slug, id string, primary bool) *domainResponse {
	a.h.t.Helper()
	rec := a.expect(http.StatusOK, "PATCH", domainsPath(slug)+"/"+id, map[string]any{"isPrimary": primary})
	d := decodeBody[domainEnvelope](a.h.t, rec).Domain
	return &d
}

func (a actor) siteByLive(slug string) siteResponse {
	a.h.t.Helper()
	return decodeBody[siteEnvelope](a.h.t, a.expect(http.StatusOK, "GET", "/api/v1/sites/"+slug, nil)).Site
}

func (h *harness) advance(d time.Duration) {
	h.clock.Advance(d)
}
