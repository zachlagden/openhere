package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/zachlagden/openhere/internal/analytics"
	"github.com/zachlagden/openhere/internal/analytics/geo"
	"github.com/zachlagden/openhere/internal/settings"
)

const settledReadTimeout = 10 * time.Second

type GeoChecker interface {
	ForceCheck(ctx context.Context) error
}

type geoCheckResponse struct {
	Status    string     `json:"status"`
	CheckedAt *time.Time `json:"checkedAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
	Detail    string     `json:"detail"`
}

func (s *Server) registerAnalyticsRoutes(mux *routeMux) {
	mux.Handle("GET "+basePath+"/sites/{slug}/analytics", s.protected(s.handleSiteAnalytics))
	mux.Handle("GET "+basePath+"/workspaces/{workspace}/analytics", s.protected(s.handleWorkspaceAnalytics))
	mux.Handle("GET "+basePath+"/admin/analytics", s.protected(s.handleInstanceAnalytics))
	mux.Handle("POST "+basePath+"/admin/analytics/geo/check", s.protected(s.handleGeoCheck))
}

func (s *Server) handleGeoCheck(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	if s.geo == nil {
		writeError(w, http.StatusNotFound, codeNotFound, "Not found.")
		return
	}
	before, err := s.settings.Get(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	if before.Analytics.MaxMindAccountID == "" || before.Analytics.MaxMindLicenseKey == "" {
		writeError(w, http.StatusConflict, codeGeoNotConfigured, "Save a MaxMind account ID and licence key before checking the location database.")
		return
	}
	if err := s.geo.ForceCheck(r.Context()); err != nil {
		var soon *geo.TooSoonError
		if errors.As(err, &soon) {
			writeGeoTooSoon(w, soon)
			return
		}
		s.writeInternal(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), settledReadTimeout)
	defer cancel()
	after, err := s.settings.Get(ctx)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toGeoCheckResponse(after))
}

func writeGeoTooSoon(w http.ResponseWriter, soon *geo.TooSoonError) {
	seconds := int(math.Ceil(soon.RetryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, errorBody{
		Code:       codeRateLimited,
		Message:    "A location database check ran less than a minute ago. Try again in " + strconv.Itoa(seconds) + " seconds.",
		RetryAfter: &seconds,
	})
}

func toGeoCheckResponse(cfg settings.Settings) geoCheckResponse {
	response := geoCheckResponse{Status: cfg.Analytics.GeoStatus, Detail: cfg.Analytics.GeoDetail}
	if checked, err := time.Parse(time.RFC3339, cfg.Analytics.GeoCheckedAt); err == nil {
		response.CheckedAt = &checked
	}
	if updated, err := time.Parse(time.RFC3339, cfg.Analytics.GeoUpdatedAt); err == nil {
		response.UpdatedAt = &updated
	}
	return response
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
