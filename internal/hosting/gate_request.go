package hosting

import (
	"net/http"
	"net/url"
	"strings"
)

type siteRequest struct {
	w      http.ResponseWriter
	r      *http.Request
	site   Site
	path   string
	prefix string
}

func (s siteRequest) pathMode() bool {
	return s.prefix != ""
}

func (s siteRequest) returnTarget() string {
	escaped := strings.TrimPrefix(s.r.URL.EscapedPath(), s.prefix)
	target := url.URL{Path: s.path, RawPath: escaped, RawQuery: s.r.URL.RawQuery}
	return target.RequestURI()
}

func (s siteRequest) redirectTo(returnPath string) {
	s.w.Header().Set("Location", s.prefix+returnPath)
	s.w.Header().Set("Cache-Control", cacheGatePage)
	s.w.WriteHeader(http.StatusSeeOther)
}
