package api

import (
	"time"

	"github.com/zachlagden/openhere/internal/domains"
)

type challengeResponse struct {
	RecordType string `json:"recordType"`
	Name       string `json:"name"`
	Value      string `json:"value"`
}

type targetResponse struct {
	RecordType string `json:"recordType"`
	Value      string `json:"value"`
	Note       string `json:"note"`
}

type domainResponse struct {
	ID              string            `json:"id"`
	Domain          string            `json:"domain"`
	Status          string            `json:"status"`
	Challenge       challengeResponse `json:"challenge"`
	Target          targetResponse    `json:"target"`
	IsPrimary       bool              `json:"isPrimary"`
	VerifiedAt      *time.Time        `json:"verifiedAt"`
	LastCheckedAt   *time.Time        `json:"lastCheckedAt"`
	LastCheckResult *string           `json:"lastCheckResult"`
	CreatedAt       time.Time         `json:"createdAt"`
}

type domainEnvelope struct {
	Domain domainResponse `json:"domain"`
}

type domainListResponse struct {
	Domains []domainResponse `json:"domains"`
}

type domainVerifyResponse struct {
	Domain   domainResponse `json:"domain"`
	Verified bool           `json:"verified"`
	Message  string         `json:"message"`
}

func (s *Server) toDomainResponse(d domains.Domain) domainResponse {
	return domainResponse{
		ID: d.ID, Domain: d.Name, Status: d.Status(),
		Challenge: challengeResponse{RecordType: "TXT", Name: d.ChallengeName(), Value: d.Token},
		Target: targetResponse{
			RecordType: "CNAME", Value: s.public.Host,
			Note: "Point the domain at " + s.public.Host + " with a CNAME record. A domain with no subdomain part, such as example.com, usually cannot have a CNAME, so use A and AAAA records with the same addresses as " + s.public.Host + " instead. A domain behind a CDN or proxy can resolve elsewhere, as long as that service forwards requests for it to this server.",
		},
		IsPrimary: d.IsPrimary, VerifiedAt: d.VerifiedAt, LastCheckedAt: d.LastCheckedAt,
		LastCheckResult: optional(d.LastCheckResult), CreatedAt: d.CreatedAt,
	}
}

func (s *Server) toDomainResponses(list []domains.Domain) []domainResponse {
	out := make([]domainResponse, 0, len(list))
	for _, d := range list {
		out = append(out, s.toDomainResponse(d))
	}
	return out
}
