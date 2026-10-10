package api

import (
	"net/http"

	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type domainCreateRequest struct {
	Domain string `json:"domain"`
}

type domainUpdateRequest struct {
	IsPrimary *bool `json:"isPrimary"`
}

func (s *Server) registerDomainRoutes(mux *routeMux) {
	if s.domains == nil {
		return
	}
	base := basePath + "/sites/{slug}/domains"
	mux.Handle("GET "+base, s.protected(s.handleListDomains))
	mux.Handle("POST "+base, s.protected(s.handleCreateDomain))
	mux.Handle("GET "+base+"/{id}", s.protected(s.handleGetDomain))
	mux.Handle("POST "+base+"/{id}/verify", s.protected(s.handleVerifyDomain))
	mux.Handle("PATCH "+base+"/{id}", s.protected(s.handleUpdateDomain))
	mux.Handle("DELETE "+base+"/{id}", s.protected(s.handleDeleteDomain))
}

func (s *Server) authorizeDomainSite(w http.ResponseWriter, r *http.Request, role workspaces.Role) (sites.Site, *principal, bool) {
	p, _ := principalFrom(r.Context())
	site, err := s.sites.Authorize(r.Context(), p.user.ID, r.PathValue("slug"), role)
	if err != nil {
		s.writeDomainError(w, r, err)
		return sites.Site{}, nil, false
	}
	return site, p, true
}

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	site, _, ok := s.authorizeDomainSite(w, r, workspaces.RoleRead)
	if !ok {
		return
	}
	list, err := s.domains.List(r.Context(), site.ID)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domainListResponse{Domains: s.toDomainResponses(list)})
}

func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	site, _, ok := s.authorizeDomainSite(w, r, workspaces.RoleRead)
	if !ok {
		return
	}
	d, err := s.domains.Get(r.Context(), site.ID, r.PathValue("id"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domainEnvelope{Domain: s.toDomainResponse(d)})
}

func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	site, p, ok := s.authorizeDomainSite(w, r, workspaces.RoleWrite)
	if !ok {
		return
	}
	var req domainCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.allow(w, s.domainChange, p.user.ID) {
		return
	}
	d, err := s.domains.Claim(r.Context(), site.ID, req.Domain)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, domainEnvelope{Domain: s.toDomainResponse(d)})
}

func (s *Server) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	site, p, ok := s.authorizeDomainSite(w, r, workspaces.RoleWrite)
	if !ok {
		return
	}
	if !s.allow(w, s.domainVerify, p.user.ID) {
		return
	}
	d, err := s.domains.Verify(r.Context(), site.ID, r.PathValue("id"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domainVerifyResponse{Domain: s.toDomainResponse(d), Verified: d.Verified(), Message: d.ResultMessage()})
}

func (s *Server) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	site, p, ok := s.authorizeDomainSite(w, r, workspaces.RoleWrite)
	if !ok {
		return
	}
	var req domainUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IsPrimary == nil {
		s.writeDomainError(w, r, validate.Invalid("Send isPrimary as true or false."))
		return
	}
	if !s.allow(w, s.domainChange, p.user.ID) {
		return
	}
	d, err := s.domains.SetPrimary(r.Context(), site.ID, r.PathValue("id"), *req.IsPrimary)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domainEnvelope{Domain: s.toDomainResponse(d)})
}

func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	site, _, ok := s.authorizeDomainSite(w, r, workspaces.RoleWrite)
	if !ok {
		return
	}
	if err := s.domains.Delete(r.Context(), site.ID, r.PathValue("id")); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
