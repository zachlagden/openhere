package api

import (
	"net/http"

	"github.com/zachlagden/selfhost/internal/auth"
)

const (
	sessionCookieName       = "session"
	secureSessionCookieName = "__Host-" + sessionCookieName
)

func (s *Server) secure(r *http.Request) bool {
	return s.public.Secure() || s.proxies.isHTTPS(r)
}

func cookieNameFor(secure bool) string {
	if secure {
		return secureSessionCookieName
	}
	return sessionCookieName
}

func (s *Server) sessionCookieFrom(r *http.Request) (*http.Cookie, error) {
	return r.Cookie(cookieNameFor(s.secure(r)))
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	writeSessionCookie(w, s.secure(r), token, int(auth.SessionTTL.Seconds()))
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	writeSessionCookie(w, s.secure(r), "", -1)
}

func writeSessionCookie(w http.ResponseWriter, secure bool, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieNameFor(secure),
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
