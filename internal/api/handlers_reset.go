package api

import (
	"net/http"
	"time"

	"github.com/zachlagden/selfhost/internal/proxy"
)

type resetRequest struct {
	Email string `json:"email"`
}

type resetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type genericStatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type adminResetLinkResponse struct {
	Link      string    `json:"link"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Server) registerPasswordResetRoutes(mux *http.ServeMux) {
	mux.Handle("POST "+basePath+"/auth/password-reset/request", s.identify(http.HandlerFunc(s.handleResetRequest)))
	mux.Handle("POST "+basePath+"/auth/password-reset/confirm", s.identify(http.HandlerFunc(s.handleResetConfirm)))
}

func (s *Server) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.resetIP, proxy.LimiterKey(s.proxies.ClientIP(r))) {
		return
	}
	var req resetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.allow(w, s.resetEmail, emailLimitKey(req.Email)) {
		return
	}
	if err := s.reset.Request(r.Context(), req.Email); err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, genericStatusResponse{
		Status:  "ok",
		Message: "If that address belongs to an account and this instance can send email, a reset link is on its way.",
	})
}

func (s *Server) handleResetConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.resetConfirm, proxy.LimiterKey(s.proxies.ClientIP(r))) {
		return
	}
	var req resetConfirmRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.reset.Confirm(r.Context(), req.Token, req.Password); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminResetLink(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	link, expires, err := s.reset.AdminLink(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, adminResetLinkResponse{Link: link, ExpiresAt: expires})
}
