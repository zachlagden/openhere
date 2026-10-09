package api

import (
	"errors"
	"net/http"

	"github.com/zachlagden/selfhost/internal/auth"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type meResponse struct {
	User       userResponse `json:"user"`
	AuthMethod authMethod   `json:"authMethod"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.loginIP, limiterKeyForIP(s.proxies.clientIP(r))) {
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	emailKey := emailLimitKey(req.Email)
	if !s.allow(w, s.loginEmail, emailKey) {
		return
	}
	user, err := s.auth.Authenticate(r.Context(), req.Email, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, codeInvalidLogin, "Incorrect email or password.")
		return
	}
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	s.loginEmail.Reset(emailKey)
	s.startSession(w, r, user, http.StatusOK)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user auth.User, status int) {
	token, _, err := s.auth.CreateSession(r.Context(), user.ID)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, status, userEnvelope{User: toUserResponse(user)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if p.method == methodSession {
		if err := s.auth.DeleteSession(r.Context(), p.sessionToken); err != nil {
			s.writeInternal(w, r, err)
			return
		}
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	writeJSON(w, http.StatusOK, meResponse{User: toUserResponse(p.user), AuthMethod: p.method})
}
