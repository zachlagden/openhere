package hosting

import (
	"context"
	"net"
	"sync"

	"github.com/zachlagden/selfhost/internal/domains"
)

type fakeDNS struct {
	mu      sync.Mutex
	answers map[string][]string
}

func (f *fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func (e *env) claimDomain(slug, name string) domains.Domain {
	e.t.Helper()
	d, err := e.domains.Claim(context.Background(), e.siteIDOf(slug), name)
	if err != nil {
		e.t.Fatalf("claim %s: %v", name, err)
	}
	return d
}

func (e *env) verifiedDomain(slug, name string) domains.Domain {
	e.t.Helper()
	d := e.claimDomain(slug, name)
	e.dns.publish(d.ChallengeName(), d.Token)
	verified, err := e.domains.Verify(context.Background(), d.SiteID, d.ID)
	if err != nil || !verified.Verified() {
		e.t.Fatalf("verify %s: %+v, %v", name, verified, err)
	}
	return verified
}
