package api

import (
	"log/slog"
	"net/http"

	"github.com/zachlagden/selfhost/docs"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/hosting"
	"github.com/zachlagden/selfhost/internal/policy"
)

type instanceModesResponse struct {
	Subdomain     bool `json:"subdomain"`
	Path          bool `json:"path"`
	CustomDomains bool `json:"customDomains"`
}

type instanceResponse struct {
	PublicURL            string                `json:"publicUrl"`
	Version              string                `json:"version"`
	EmailAvailable       bool                  `json:"emailAvailable"`
	CanCreateWorkspaces  bool                  `json:"canCreateWorkspaces"`
	HostingModes         instanceModesResponse `json:"hostingModes"`
	MaxDomainsPerSite    int                   `json:"maxDomainsPerSite"`
	WhoCanInvite         string                `json:"whoCanInvite"`
	AgentCredentialsDir  string                `json:"agentCredentialsDir"`
	AgentCredentialsPath string                `json:"agentCredentialsPath"`
}

func (s *Server) registerInstanceRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+basePath+"/instance", s.protected(s.handleInstance))
	mux.Handle("GET "+basePath+"/docs/openapi.yaml", s.protected(s.handleDocsOpenAPI))
	mux.Handle("GET "+basePath+"/docs/agent-guide.md", s.protected(s.handleDocsAgentGuide))
}

func (s *Server) handleInstance(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	cfg, err := s.settings.Get(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	availability := hosting.InstanceHosting(cfg)
	writeJSON(w, http.StatusOK, instanceResponse{
		PublicURL:           s.public.String(),
		Version:             s.version,
		EmailAvailable:      s.mailer.Configured(),
		CanCreateWorkspaces: policy.CanCreateShared(cfg, p.user),
		HostingModes: instanceModesResponse{
			Subdomain:     availability.Subdomain.State == hosting.StateOn,
			Path:          availability.Path.State == hosting.StateOn,
			CustomDomains: availability.CustomDomains.State == hosting.StateOn,
		},
		MaxDomainsPerSite:    cfg.MaxDomainsPerSite,
		WhoCanInvite:         cfg.WhoCanInvite,
		AgentCredentialsDir:  config.AgentCredentialsDir,
		AgentCredentialsPath: config.AgentCredentialsPath(),
	})
}

func (s *Server) handleDocsOpenAPI(w http.ResponseWriter, _ *http.Request) {
	writeDocument(w, "application/yaml; charset=utf-8", docs.OpenAPI)
}

func (s *Server) handleDocsAgentGuide(w http.ResponseWriter, _ *http.Request) {
	writeDocument(w, "text/markdown; charset=utf-8", docs.AgentGuide)
}

func writeDocument(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		slog.Debug("write document", "error", err)
	}
}
