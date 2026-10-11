package api

import (
	"net/http"

	"github.com/zachlagden/openhere/internal/analytics"
)

func (s *Server) registerAnalyticsRoutes(mux *routeMux) {
	mux.Handle("GET "+basePath+"/sites/{slug}/analytics", s.protected(s.handleSiteAnalytics))
	mux.Handle("GET "+basePath+"/workspaces/{workspace}/analytics", s.protected(s.handleWorkspaceAnalytics))
	mux.Handle("GET "+basePath+"/admin/analytics", s.protected(s.handleInstanceAnalytics))
}

func (s *Server) handleSiteAnalytics(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	params, ok := s.analyticsParams(w, r)
	if !ok {
		return
	}
	report, err := s.analytics.SiteReport(r.Context(), p.user.ID, r.PathValue("slug"), params)
	s.writeAnalytics(w, r, report, err)
}

func (s *Server) handleWorkspaceAnalytics(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	params, ok := s.analyticsParams(w, r)
	if !ok {
		return
	}
	report, err := s.analytics.WorkspaceReport(r.Context(), p.user.ID, r.PathValue("workspace"), params)
	s.writeAnalytics(w, r, report, err)
}

func (s *Server) handleInstanceAnalytics(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	params, ok := s.analyticsParams(w, r)
	if !ok {
		return
	}
	report, err := s.analytics.InstanceReport(r.Context(), params)
	s.writeAnalytics(w, r, report, err)
}

func (s *Server) analyticsParams(w http.ResponseWriter, r *http.Request) (analytics.Params, bool) {
	params, err := s.analytics.ParseParams(r.URL.Query())
	if err != nil {
		s.writeDomainError(w, r, err)
		return analytics.Params{}, false
	}
	return params, true
}

func (s *Server) writeAnalytics(w http.ResponseWriter, r *http.Request, report analytics.Report, err error) {
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAnalyticsResponse(report))
}
