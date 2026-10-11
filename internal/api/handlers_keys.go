package api

import (
	"errors"
	"net/http"

	"github.com/zachlagden/openhere/internal/auth"
)

type createKeyRequest struct {
	Label string `json:"label"`
}

type keyListResponse struct {
	Keys []apiKeyResponse `json:"keys"`
}

type createKeyResponse struct {
	Key    string         `json:"key"`
	APIKey apiKeyResponse `json:"apiKey"`
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	keys, err := s.auth.ListAPIKeys(r.Context(), p.user.ID)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	response := keyListResponse{Keys: make([]apiKeyResponse, 0, len(keys))}
	for _, key := range keys {
		response.Keys = append(response.Keys, toAPIKeyResponse(key))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req createKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	secret, record, err := s.auth.CreateAPIKey(r.Context(), p.user.ID, req.Label)
	if errors.Is(err, auth.ErrAPIKeyLimit) {
		writeError(w, http.StatusConflict, codeKeyLimitReached, "You have reached the limit of 50 active API keys. Revoke one first.")
		return
	}
	if s.writeAuthError(w, r, err) {
		return
	}
	writeJSON(w, http.StatusCreated, createKeyResponse{Key: secret, APIKey: toAPIKeyResponse(record)})
}

func (s *Server) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	err := s.auth.RevokeAPIKey(r.Context(), p.user.ID, r.PathValue("id"))
	if errors.Is(err, auth.ErrAPIKeyNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "API key not found.")
		return
	}
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
