package api

import (
	"time"

	"github.com/zachlagden/selfhost/internal/certs"
	"github.com/zachlagden/selfhost/internal/hosting"
	"github.com/zachlagden/selfhost/internal/settings"
)

type modeResponse struct {
	State  string  `json:"state"`
	Reason *string `json:"reason"`
}

type modesResponse struct {
	Subdomain     modeResponse `json:"subdomain"`
	Path          modeResponse `json:"path"`
	CustomDomains modeResponse `json:"customDomains"`
}

type siteHostingResponse struct {
	Reachable bool             `json:"reachable"`
	URL       *string          `json:"url"`
	Modes     modesResponse    `json:"modes"`
	Notices   []noticeResponse `json:"notices"`
}

type hostingDetailResponse struct {
	WildcardStatus          string            `json:"wildcardStatus"`
	WildcardCheckedAt       *time.Time        `json:"wildcardCheckedAt"`
	WildcardDetail          string            `json:"wildcardDetail"`
	ConsecutiveFailures     int               `json:"consecutiveFailures"`
	SubdomainDisabledReason *string           `json:"subdomainDisabledReason"`
	TLS                     tlsDetailResponse `json:"tls"`
}

type hostingStatusResponse struct {
	Degraded bool                   `json:"degraded"`
	Modes    modesResponse          `json:"modes"`
	Notices  []noticeResponse       `json:"notices"`
	Detail   *hostingDetailResponse `json:"detail,omitempty"`
}

func toModeResponse(m hosting.ModeStatus) modeResponse {
	return modeResponse{State: m.State, Reason: optional(m.Reason)}
}

func toModesResponse(subdomain, path, custom hosting.ModeStatus) modesResponse {
	return modesResponse{Subdomain: toModeResponse(subdomain), Path: toModeResponse(path), CustomDomains: toModeResponse(custom)}
}

func toNoticeResponses(notices []hosting.Notice) []noticeResponse {
	out := make([]noticeResponse, 0, len(notices))
	for _, n := range notices {
		out = append(out, noticeResponse{Code: n.Code, Message: n.Message})
	}
	return out
}

func toSiteHostingResponse(a hosting.SiteAvailability) siteHostingResponse {
	return siteHostingResponse{
		Reachable: a.Reachable, URL: optional(a.URL), Modes: toModesResponse(a.Subdomain, a.Path, a.CustomDomains), Notices: toNoticeResponses(a.Notices),
	}
}

func toHostingStatusResponse(a hosting.InstanceAvailability) hostingStatusResponse {
	return hostingStatusResponse{
		Degraded: a.Degraded, Modes: toModesResponse(a.Subdomain, a.Path, a.CustomDomains), Notices: toNoticeResponses(a.Notices),
	}
}

func toHostingDetailResponse(cfg settings.Settings, tlsStatus certs.Status) *hostingDetailResponse {
	detail := &hostingDetailResponse{
		WildcardStatus: cfg.WildcardStatus, WildcardDetail: cfg.WildcardDetail, ConsecutiveFailures: cfg.WildcardFailures,
		SubdomainDisabledReason: optional(cfg.SubdomainDisabledReason),
		TLS:                     toTLSDetailResponse(tlsStatus, cfg),
	}
	if checked, err := time.Parse(time.RFC3339, cfg.WildcardCheckedAt); err == nil {
		detail.WildcardCheckedAt = &checked
	}
	return detail
}
