package api

import (
	"net/http"
	"strconv"

	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/validate"
)

type createSiteRequest struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	FolderID    string `json:"folderId"`
	SpaMode     bool   `json:"spaMode"`
}

type updateSiteRequest struct {
	DisplayName *string        `json:"displayName"`
	Description *string        `json:"description"`
	SpaMode     *bool          `json:"spaMode"`
	Analytics   *bool          `json:"analyticsEnabled"`
	FolderID    optionalString `json:"folderId"`
}

func (s *Server) registerSiteRoutes(mux *routeMux) {
	mux.Handle("GET "+basePath+"/sites", s.protected(s.handleListSites))
	mux.Handle("POST "+basePath+"/workspaces/{workspace}/sites", s.protected(s.handleCreateSite))
	mux.Handle("GET "+basePath+"/sites/{slug}", s.protected(s.handleGetSite))
	mux.Handle("PATCH "+basePath+"/sites/{slug}", s.protected(s.handleUpdateSite))
	mux.Handle("DELETE "+basePath+"/sites/{slug}", s.protected(s.handleDeleteSite))
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	filter, err := parseSiteFilter(r)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	page, err := s.sites.ListSites(r.Context(), p.user.ID, filter)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	hostingFor, err := s.siteHosting(r.Context(), page.Sites...)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	response := siteListResponse{Sites: make([]siteResponse, 0, len(page.Sites)), NextCursor: optional(page.NextCursor)}
	for _, site := range page.Sites {
		response.Sites = append(response.Sites, toSiteResponse(site, hostingFor(site)))
	}
	writeJSON(w, http.StatusOK, response)
}

func parseSiteFilter(r *http.Request) (sites.Filter, error) {
	query := r.URL.Query()
	filter := sites.Filter{
		Workspace: query.Get("workspace"),
		Folder:    query.Get("folder"),
		Search:    query.Get("q"),
		Cursor:    query.Get("cursor"),
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return sites.Filter{}, validate.Invalid("Limit must be a positive whole number.")
		}
		filter.Limit = limit
	}
	return filter, nil
}

func (s *Server) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req createSiteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.sites.CreateSite(r.Context(), sites.CreateInput{
		UserID: p.user.ID, Workspace: r.PathValue("workspace"), Slug: req.Slug, DisplayName: req.DisplayName,
		Description: req.Description, FolderID: req.FolderID, SpaMode: req.SpaMode,
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	s.writeSite(w, r, http.StatusCreated, site)
}

func (s *Server) handleGetSite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	site, err := s.sites.GetSite(r.Context(), p.user.ID, r.PathValue("slug"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	s.writeSite(w, r, http.StatusOK, site)
}

func (s *Server) handleUpdateSite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req updateSiteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	update := sites.UpdateInput{DisplayName: req.DisplayName, Description: req.Description, SpaMode: req.SpaMode, Analytics: req.Analytics}
	if req.FolderID.Set {
		update.Folder.Set = true
		if req.FolderID.Value != nil {
			update.Folder.ID = *req.FolderID.Value
		}
	}
	site, err := s.sites.UpdateSite(r.Context(), p.user.ID, r.PathValue("slug"), update)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	s.writeSite(w, r, http.StatusOK, site)
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.sites.DeleteSite(r.Context(), p.user.ID, r.PathValue("slug")); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeSite(w http.ResponseWriter, r *http.Request, status int, site sites.Site) {
	hostingFor, err := s.siteHosting(r.Context(), site)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, status, siteEnvelope{Site: toSiteResponse(site, hostingFor(site))})
}
