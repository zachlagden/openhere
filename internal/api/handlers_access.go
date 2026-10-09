package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/hosting"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	maxAccessBodyBytes = 512 << 10
	loginPath          = "/login"
)

type accessUpdateRequest struct {
	Mode            *string        `json:"mode"`
	Password        optionalString `json:"password"`
	EmailAllowlist  *[]string      `json:"emailAllowlist"`
	DomainAllowlist *[]string      `json:"domainAllowlist"`
}

type accessResponse struct {
	Mode               string   `json:"mode"`
	HasPassword        bool     `json:"hasPassword"`
	EmailAllowlist     []string `json:"emailAllowlist"`
	DomainAllowlist    []string `json:"domainAllowlist"`
	EmailCodeAvailable bool     `json:"emailCodeAvailable"`
}

type accessEnvelope struct {
	Access accessResponse `json:"access"`
}

func (s *Server) registerAccessRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+basePath+"/publish/{slug}/access", s.protected(s.handleGetAccess))
	mux.Handle("PATCH "+basePath+"/publish/{slug}/access", s.protected(s.handleUpdateAccess))
	mux.Handle("GET "+hosting.HandoffPath, s.identify(http.HandlerFunc(s.handleHandoff)))
}

func (s *Server) toAccessResponse(cfg access.Config) accessResponse {
	return accessResponse{
		Mode: string(cfg.Mode), HasPassword: cfg.HasPassword, EmailAllowlist: cfg.Emails, DomainAllowlist: cfg.Domains,
		EmailCodeAvailable: s.access.EmailCodeAvailable(),
	}
}

func (s *Server) handleGetAccess(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	site, err := s.sites.Authorize(r.Context(), p.user.ID, r.PathValue("slug"), workspaces.RoleRead)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	cfg, err := s.access.Get(r.Context(), site.ID)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, accessEnvelope{Access: s.toAccessResponse(cfg)})
}

func (s *Server) handleUpdateAccess(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	site, err := s.sites.Authorize(r.Context(), p.user.ID, r.PathValue("slug"), workspaces.RoleWrite)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	var req accessUpdateRequest
	if !decodeJSONLimited(w, r, &req, maxAccessBodyBytes) {
		return
	}
	cfg, err := s.access.Update(r.Context(), site.ID, toConfigUpdate(req))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, accessEnvelope{Access: s.toAccessResponse(cfg)})
}

func toConfigUpdate(req accessUpdateRequest) access.ConfigUpdate {
	return access.ConfigUpdate{
		Mode:     req.Mode,
		Password: access.PasswordChange{Set: req.Password.Set, Value: req.Password.Value},
		Emails:   req.EmailAllowlist,
		Domains:  req.DomainAllowlist,
	}
}

func (s *Server) handleHandoff(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	returnPath, ok := handoffReturn(query.Get("return"))
	if !ok {
		s.writeDomainError(w, r, validate.Invalid("Return must be a path on the site, such as /docs/."))
		return
	}
	p, signedIn := principalFrom(r.Context())
	if !signedIn {
		redirect(w, loginPath+"?"+url.Values{"next": {r.URL.RequestURI()}}.Encode())
		return
	}
	if !s.allow(w, s.handoffLimit, p.user.ID) {
		return
	}
	site, err := s.sites.Authorize(r.Context(), p.user.ID, query.Get("site"), workspaces.RoleRead)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	target, err := s.handoffTarget(r, site, p.user.ID, returnPath)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	redirect(w, target)
}

func handoffReturn(raw string) (string, bool) {
	if raw == "" {
		return "/", true
	}
	return access.SafeReturnPath(raw)
}

func (s *Server) handoffTarget(r *http.Request, site sites.Site, userID, returnPath string) (string, error) {
	cfg, err := s.settings.Get(r.Context())
	if err != nil {
		return "", err
	}
	base := hosting.SiteURL(s.public, cfg, site.Slug)
	if base == "" {
		return "", workspaces.ErrNotFound
	}
	if access.Mode(site.AccessMode) != access.ModeRestricted {
		return base + strings.TrimPrefix(returnPath, "/"), nil
	}
	token, err := s.access.IssueHandoff(r.Context(), site.ID, userID)
	if err != nil {
		return "", err
	}
	query := url.Values{"token": {token}, "return": {returnPath}}
	return base + strings.TrimPrefix(hosting.CallbackPath, "/") + "?" + query.Encode(), nil
}

func redirect(w http.ResponseWriter, target string) {
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusSeeOther)
}
