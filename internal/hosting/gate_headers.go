package hosting

import (
	"net/http"
	"strings"
)

const (
	frameAncestorsNone = "frame-ancestors 'none'"
	gatePagePolicy     = "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; " + frameAncestorsNone
	cacheGatePage      = "private, no-store"
	cachePrivateFile   = "private, no-cache"
)

func setGatedHeaders(w http.ResponseWriter, pathMode, page bool) {
	h := w.Header()
	policy := frameAncestorsNone
	if page {
		policy = gatePagePolicy
	}
	if pathMode {
		policy = SandboxPolicy + "; " + policy
	}
	h.Set("Content-Security-Policy", policy)
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", cacheGatePage)
	w.WriteHeader(http.StatusUnauthorized)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write([]byte("Unauthorized\n")); err != nil {
		logWriteFailure(err)
	}
}

func isNavigation(r *http.Request) bool {
	if dest := r.Header.Get("Sec-Fetch-Dest"); dest != "" {
		switch dest {
		case "document", "iframe", "frame", "fencedframe":
			return true
		default:
			return false
		}
	}
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}
