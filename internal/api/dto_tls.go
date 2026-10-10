package api

import (
	"context"
	"time"

	"github.com/zachlagden/selfhost/internal/certs"
	"github.com/zachlagden/selfhost/internal/settings"
)

const (
	noticeTLSWildcard  = "tls_wildcard_unavailable"
	messageTLSWildcard = "Subdomain addresses may show certificate warnings."
)

type TLSReporter interface {
	Status(ctx context.Context) (certs.Status, error)
}

type wildcardCertificateResponse struct {
	State  string  `json:"state"`
	Reason *string `json:"reason"`
}

type customCertificatesResponse struct {
	Count         int        `json:"count"`
	SoonestExpiry *time.Time `json:"soonestExpiresAt"`
}

type tlsDetailResponse struct {
	Enabled                        bool                        `json:"enabled"`
	ManagementCertificateExpiresAt *time.Time                  `json:"managementCertificateExpiresAt"`
	Wildcard                       wildcardCertificateResponse `json:"wildcard"`
	CustomDomains                  customCertificatesResponse  `json:"customDomains"`
	DNSProviderConfigured          bool                        `json:"dnsProviderConfigured"`
}

func toTLSDetailResponse(status certs.Status, cfg settings.Settings) tlsDetailResponse {
	state := status.Wildcard.State
	if state == "" {
		state = certs.WildcardNone
	}
	configured := status.DNSProviderConfigured
	if !status.Enabled {
		configured = cfg.TLS.DNSProvider != settings.DNSProviderNone && cfg.TLS.CloudflareAPIToken != ""
	}
	return tlsDetailResponse{
		Enabled:                        status.Enabled,
		ManagementCertificateExpiresAt: status.ManagementExpiresAt,
		Wildcard:                       wildcardCertificateResponse{State: state, Reason: optional(status.Wildcard.Reason)},
		CustomDomains:                  customCertificatesResponse{Count: status.Custom.Count, SoonestExpiry: status.Custom.SoonestExpiry},
		DNSProviderConfigured:          configured,
	}
}

func tlsNotices(status certs.Status, cfg settings.Settings) []noticeResponse {
	if !status.Enabled || !cfg.SubdomainEnabled {
		return nil
	}
	if status.Wildcard.State == certs.WildcardOK || status.Wildcard.State == certs.WildcardPending {
		return nil
	}
	return []noticeResponse{{Code: noticeTLSWildcard, Message: messageTLSWildcard}}
}
