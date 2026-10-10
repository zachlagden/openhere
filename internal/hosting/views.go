package hosting

import (
	"net/http"
	"strings"

	"github.com/zachlagden/selfhost/internal/analytics"
)

type ViewRecorder interface {
	Record(hit analytics.Hit)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(body []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(body)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func (s siteServer) countView(r *http.Request, site Site, requestPath, contentType string, status int) {
	if s.views == nil || !site.Analytics || !isCountedRequest(r, contentType, status) {
		return
	}
	s.views.Record(analytics.Hit{
		SiteID: site.ID, Path: requestPath, Status: status, Referrer: r.Header.Get("Referer"), Host: r.Host,
		Addr: s.proxies.ClientIP(r), UserAgent: r.Header.Get("User-Agent"),
	})
}

func isCountedRequest(r *http.Request, contentType string, status int) bool {
	if r.Method != http.MethodGet || r.Header.Get("Range") != "" || !isHTML(contentType) {
		return false
	}
	if status != http.StatusOK && status != http.StatusNotFound {
		return false
	}
	return !isPrefetch(r)
}

func isPrefetch(r *http.Request) bool {
	for _, name := range []string{"Sec-Purpose", "Purpose"} {
		if strings.Contains(strings.ToLower(r.Header.Get(name)), "prefetch") {
			return true
		}
	}
	return false
}
