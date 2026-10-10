package api

import (
	"net/http"

	"github.com/zachlagden/selfhost/internal/hosting"
)

func (s *Server) handleHostingStatus(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	cfg, err := s.settings.Get(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	response := toHostingStatusResponse(hosting.InstanceHosting(cfg))
	if p.user.IsRoot {
		response.Detail = toHostingDetailResponse(cfg)
	}
	writeJSON(w, http.StatusOK, response)
}
