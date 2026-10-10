package api

import (
	"net/http"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/proxy"
)

const (
	agentNoEmailMessage = "This instance cannot send email, so email sign-in is not available. Ask the user to create an API key in the dashboard under Settings, API keys, and give it to you."
	globalLimitKey      = "global"
)

type agentRequestCode struct {
	Email string `json:"email"`
}

type agentKeyResponse struct {
	Key             string         `json:"key"`
	APIKey          apiKeyResponse `json:"apiKey"`
	CredentialsPath string         `json:"credentialsPath"`
}

type agentVerifyCode struct {
	Email string `json:"email"`
	Code  string `json:"code"`
	Label string `json:"label"`
}

func (s *Server) registerAgentRoutes(mux *routeMux) {
	mux.Handle("POST "+basePath+"/auth/agent/request-code", s.identify(http.HandlerFunc(s.handleAgentRequestCode)))
	mux.Handle("POST "+basePath+"/auth/agent/verify-code", s.identify(http.HandlerFunc(s.handleAgentVerifyCode)))
}

func (s *Server) handleAgentRequestCode(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.agentReqIP, proxy.LimiterKey(s.proxies.ClientIP(r))) {
		return
	}
	var req agentRequestCode
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.agents.Available() {
		writeError(w, http.StatusConflict, codeEmailNotConfigured, agentNoEmailMessage)
		return
	}
	if !s.allow(w, s.agentReqEmail, emailLimitKey(req.Email)) || !s.allow(w, s.agentReqAll, globalLimitKey) {
		return
	}
	if err := s.agents.Request(r.Context(), req.Email); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, genericStatusResponse{
		Status:  "ok",
		Message: "If that address belongs to an account, an 8-digit code is on its way. Ask the user for the code, then call verify-code.",
	})
}

func (s *Server) handleAgentVerifyCode(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.agentVerifyIP, proxy.LimiterKey(s.proxies.ClientIP(r))) {
		return
	}
	var req agentVerifyCode
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.allow(w, s.agentVerifyKey, emailLimitKey(req.Email)) {
		return
	}
	secret, record, err := s.agents.Verify(r.Context(), req.Email, req.Code, req.Label)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, agentKeyResponse{Key: secret, APIKey: toAPIKeyResponse(record), CredentialsPath: config.AgentCredentialsPath()})
}
