package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/hosting"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sites"
)

type WildcardChecker interface {
	Check(ctx context.Context) (settings.Settings, error)
}

type wildcardResponse struct {
	Status    string     `json:"status"`
	CheckedAt *time.Time `json:"checkedAt"`
	Detail    string     `json:"detail"`
}

type wildcardEnvelope struct {
	Wildcard wildcardResponse `json:"wildcard"`
	Settings map[string]any   `json:"settings"`
}

func (s *Server) registerHostingRoutes(mux *http.ServeMux) {
	mux.Handle("POST "+basePath+"/admin/hosting/check", s.protected(s.handleWildcardCheck))
	mux.Handle("GET "+basePath+"/hosting/status", s.protected(s.handleHostingStatus))
}

func (s *Server) handleWildcardCheck(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if !requireRoot(w, r) {
		return
	}
	if s.wildcard == nil {
		writeError(w, http.StatusNotFound, codeNotFound, "Not found.")
		return
	}
	if !s.allow(w, s.wildcardLimit, p.user.ID) {
		return
	}
	result, err := s.wildcard.Check(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wildcardEnvelope{Wildcard: toWildcardResponse(result), Settings: settings.Values(result)})
}

func toWildcardResponse(cfg settings.Settings) wildcardResponse {
	response := wildcardResponse{Status: cfg.WildcardStatus, Detail: cfg.WildcardDetail}
	if checked, err := time.Parse(time.RFC3339, cfg.WildcardCheckedAt); err == nil {
		response.CheckedAt = &checked
	}
	return response
}

func (s *Server) siteHosting(ctx context.Context) (func(site sites.Site) hosting.SiteAvailability, error) {
	cfg, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	return func(site sites.Site) hosting.SiteAvailability {
		facts := hosting.SiteFacts{Slug: site.Slug, Live: site.CurrentVersionID != "", Gated: access.Mode(site.AccessMode).Gated()}
		return hosting.SiteHosting(s.public, cfg, facts)
	}, nil
}
