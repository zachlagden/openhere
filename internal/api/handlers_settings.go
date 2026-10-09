package api

import (
	"encoding/json"
	"net/http"

	"github.com/zachlagden/selfhost/internal/settings"
)

type settingsEnvelope struct {
	Settings map[string]any `json:"settings"`
}

type updateSettingsRequest struct {
	Settings map[string]json.RawMessage `json:"settings"`
}

func (s *Server) registerSettingsRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+basePath+"/admin/settings", s.protected(s.handleGetSettings))
	mux.Handle("PATCH "+basePath+"/admin/settings", s.protected(s.handleUpdateSettings))
}

func requireRoot(w http.ResponseWriter, r *http.Request) bool {
	p, _ := principalFrom(r.Context())
	if !p.user.IsRoot {
		writeError(w, http.StatusNotFound, codeNotFound, "Not found.")
		return false
	}
	return true
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	current, err := s.settings.Get(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsEnvelope{Settings: settings.Values(current)})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	var req updateSettingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	updated, err := s.settings.Update(r.Context(), req.Settings)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsEnvelope{Settings: settings.Values(updated)})
}
