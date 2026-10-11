package hosting

import (
	"log/slog"
	"net/http"
)

const (
	SandboxPolicy = "sandbox allow-scripts allow-forms allow-popups allow-downloads allow-modals allow-presentation"

	allowedMethods = "GET, HEAD"
)

func setSiteHeaders(w http.ResponseWriter, sandboxed bool) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	if sandboxed {
		h.Set("Content-Security-Policy", SandboxPolicy)
		h.Set("Access-Control-Allow-Origin", "*")
	}
}

func writeText(w http.ResponseWriter, status int, body string, withBody bool) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if !withBody {
		return
	}
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Debug("write response body", "error", err)
	}
}

func writeNotFound(w http.ResponseWriter, r *http.Request) {
	writeText(w, http.StatusNotFound, "Not found\n", r.Method != http.MethodHead)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", allowedMethods)
	writeText(w, http.StatusMethodNotAllowed, "Method not allowed\n", true)
}

func writeServerError(w http.ResponseWriter) {
	writeText(w, http.StatusInternalServerError, "Internal error\n", true)
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func logWriteFailure(err error) {
	slog.Debug("write response body", "error", err)
}
