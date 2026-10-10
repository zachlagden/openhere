package certs

import (
	"context"
	"crypto/x509"

	"github.com/caddyserver/certmagic"
)

const totalKey = "all"

func (p *Policy) Reserve(ctx context.Context, name string) error {
	name = normaliseName(name)
	target, serves, err := p.domains.Lookup(ctx, name)
	if err != nil {
		return errUnavailable
	}
	if !serves {
		return errNotVerified
	}
	p.reserve.Lock()
	defer p.reserve.Unlock()
	if full, _ := p.perName.Full(name); full {
		return errRateLimited
	}
	if full, _ := p.perSite.Full(target.SiteID); full {
		return errRateLimited
	}
	if full, _ := p.total.Full(totalKey); full {
		return errRateLimited
	}
	p.perName.Record(name)
	p.perSite.Record(target.SiteID)
	p.total.Record(totalKey)
	return nil
}

type limitedIssuer struct {
	inner  *certmagic.ACMEIssuer
	policy *Policy
}

func (i limitedIssuer) Issue(ctx context.Context, request *x509.CertificateRequest) (*certmagic.IssuedCertificate, error) {
	for _, name := range request.DNSNames {
		if err := i.policy.Reserve(ctx, name); err != nil {
			i.policy.refusals.note(err.Error())
			return nil, err
		}
	}
	return i.inner.Issue(ctx, request)
}

func (i limitedIssuer) IssuerKey() string {
	return i.inner.IssuerKey()
}

func (i limitedIssuer) PreCheck(ctx context.Context, names []string, interactive bool) error {
	return i.inner.PreCheck(ctx, names, interactive)
}

func (i limitedIssuer) Revoke(ctx context.Context, cert certmagic.CertificateResource, reason int) error {
	return i.inner.Revoke(ctx, cert, reason)
}
