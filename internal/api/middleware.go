package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/zachlagden/selfhost/internal/auth"
)

type authMethod string

const (
	methodSession authMethod = "session"
	methodAPIKey  authMethod = "apiKey"
)

type principal struct {
	user         auth.User
	method       authMethod
	sessionToken string
}

type principalKey struct{}

func principalFrom(ctx context.Context) (*principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*principal)
	return p, ok && p != nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) identify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.resolvePrincipal(w, r)
		if err != nil {
			s.writeInternal(w, r, err)
			return
		}
		if p != nil {
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) protected(handler http.HandlerFunc) http.Handler {
	return s.identify(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, codeUnauthenticated, "Authentication required.")
			return
		}
		if p.method == methodSession && isStateChanging(r.Method) && !sameOriginRequest(r) {
			writeError(w, http.StatusForbidden, codeCSRFRejected, "Cross-site request rejected.")
			return
		}
		handler(w, r)
	}))
}

func (s *Server) resolvePrincipal(w http.ResponseWriter, r *http.Request) (*principal, error) {
	if header := r.Header.Get("Authorization"); header != "" {
		return s.principalFromBearer(r, header)
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, nil
	}
	return s.principalFromSession(w, r, cookie.Value)
}

func (s *Server) principalFromBearer(r *http.Request, header string) (*principal, error) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return nil, nil
	}
	user, _, err := s.auth.AuthenticateAPIKey(r.Context(), strings.TrimSpace(token))
	if errors.Is(err, auth.ErrAPIKeyInvalid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &principal{user: user, method: methodAPIKey}, nil
}

func (s *Server) principalFromSession(w http.ResponseWriter, r *http.Request, token string) (*principal, error) {
	info, err := s.auth.ValidateSession(r.Context(), token)
	if errors.Is(err, auth.ErrSessionInvalid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Extended {
		s.setSessionCookie(w, r, token)
	}
	return &principal{user: info.User, method: methodSession, sessionToken: token}, nil
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func sameOriginRequest(r *http.Request) bool {
	if r.Header.Get("X-Requested-With") == "" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}
