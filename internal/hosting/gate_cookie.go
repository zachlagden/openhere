package hosting

import (
	"net/http"

	"github.com/zachlagden/openhere/internal/access"
)

const (
	viewerCookie       = "viewer"
	hostViewerCookie   = "__Host-" + viewerCookie
	secureViewerCookie = "__Secure-" + viewerCookie
)

func viewerCookieName(pathMode, secure bool) string {
	switch {
	case pathMode:
		return secureViewerCookie
	case secure:
		return hostViewerCookie
	default:
		return viewerCookie
	}
}

func (g *gate) viewerCookies(req siteRequest) []string {
	name := viewerCookieName(req.pathMode(), g.secure(req.r))
	var values []string
	for _, cookie := range req.r.Cookies() {
		if cookie.Name == name {
			values = append(values, cookie.Value)
		}
	}
	return values
}

func (g *gate) setViewerCookie(req siteRequest, value string) {
	pathMode := req.pathMode()
	secure := g.secure(req.r)
	cookie := &http.Cookie{
		Name:     viewerCookieName(pathMode, secure),
		Value:    value,
		Path:     "/",
		MaxAge:   int(access.CookieLifetime.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if pathMode {
		cookie.Path = req.prefix + "/"
		cookie.Secure = true
		cookie.SameSite = http.SameSiteNoneMode
	}
	http.SetCookie(req.w, cookie)
}

func (g *gate) secure(r *http.Request) bool {
	return g.public.Secure() || g.proxies.IsHTTPS(r)
}
