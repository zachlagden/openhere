package api

import (
	"net/http"

	"github.com/zachlagden/selfhost/internal/certs"
	"github.com/zachlagden/selfhost/internal/hosting"
)

func (s *Server) handleHostingStatus(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	cfg, err := s.settings.Get(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	var tlsStatus certs.Status
	if s.tls != nil {
		tlsStatus, err = s.tls.Status(r.Context())
		if err != nil {
			s.writeInternal(w, r, err)
			return
		}
	}
	response := toHostingStatusResponse(hosting.InstanceHosting(cfg))
	response.Notices = append(response.Notices, tlsNotices(tlsStatus, cfg)...)
	if p.user.IsRoot {
		response.Detail = toHostingDetailResponse(cfg, tlsStatus)
	}
	writeJSON(w, http.StatusOK, response)
}
