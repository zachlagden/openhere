package api

import (
	"net/http"

	"github.com/zachlagden/selfhost/internal/auth"
)

const sessionCookieName = "session"

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	writeSessionCookie(w, s.proxies.isHTTPS(r), token, int(auth.SessionTTL.Seconds()))
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	writeSessionCookie(w, s.proxies.isHTTPS(r), "", -1)
}

func writeSessionCookie(w http.ResponseWriter, secure bool, value string, maxAge int) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	cookie.Secure = secure
	http.SetCookie(w, cookie)
}
