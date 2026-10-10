package hosting

import "net/http"

const StrictTransportSecurity = "max-age=31536000"

func (rt *Router) setHSTS(w http.ResponseWriter, r *http.Request) {
	if rt.hsts && r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", StrictTransportSecurity)
	}
}
