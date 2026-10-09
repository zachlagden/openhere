package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
)

type setupStatusResponse struct {
	SetupRequired bool `json:"setupRequired"`
}

type setupRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	SetupToken string `json:"setupToken"`
}

type userEnvelope struct {
	User userResponse `json:"user"`
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	needed, err := s.auth.SetupNeeded(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{SetupRequired: needed})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.setupIP, limiterKeyForIP(s.proxies.clientIP(r))) {
		return
	}
	var req setupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.allow(w, s.setupEmail, emailLimitKey(req.Email)) {
		return
	}
	needed, err := s.auth.SetupNeeded(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	if !needed {
		writeSetupComplete(w)
		return
	}
	if !s.gate.Check(req.SetupToken) {
		writeError(w, http.StatusForbidden, codeInvalidSetupToken, "The setup token is not valid.")
		return
	}
	user, err := s.auth.Setup(r.Context(), req.Email, req.Password)
	if s.writeAuthError(w, r, err) {
		return
	}
	s.gate.Discard()
	s.startSession(w, r, user, http.StatusCreated)
}

func writeSetupComplete(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, codeSetupComplete, "Setup has already been completed.")
}

func (s *Server) writeAuthError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var validation *auth.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, codeValidationFailed, validation.Message)
	case errors.Is(err, auth.ErrSetupComplete):
		writeSetupComplete(w)
	default:
		s.writeInternal(w, r, err)
	}
	return true
}

func emailLimitKey(email string) string {
	key := strings.ToLower(strings.TrimSpace(email))
	if len(key) > auth.MaxEmailLength {
		key = key[:auth.MaxEmailLength]
	}
	return key
}

type attemptLimiter interface {
	Allow(key string) (bool, time.Duration)
}

func (s *Server) allow(w http.ResponseWriter, l attemptLimiter, key string) bool {
	ok, wait := l.Allow(key)
	if !ok {
		writeRateLimited(w, wait)
	}
	return ok
}
