package api

import (
	"net/http"
	"net/netip"

	"github.com/zachlagden/openhere/internal/proxy"
)

type loginAttempt struct {
	pairKey  string
	emailKey string
}

func newLoginAttempt(addr netip.Addr, email string) loginAttempt {
	emailKey := emailLimitKey(email)
	return loginAttempt{pairKey: proxy.LimiterKey(addr) + "|" + emailKey, emailKey: emailKey}
}

func (s *Server) allowLogin(w http.ResponseWriter, a loginAttempt) bool {
	if full, wait := s.loginPair.Full(a.pairKey); full {
		writeRateLimited(w, wait)
		return false
	}
	if full, wait := s.loginEmail.Full(a.emailKey); full && s.loginPair.Count(a.pairKey) > 0 {
		writeRateLimited(w, wait)
		return false
	}
	return true
}

func (s *Server) recordLoginFailure(a loginAttempt) {
	s.loginPair.Record(a.pairKey)
	s.loginEmail.Record(a.emailKey)
}
