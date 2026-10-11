package hosting

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/zachlagden/openhere/internal/analytics"
	"github.com/zachlagden/openhere/internal/httptimeouts"
	"github.com/zachlagden/openhere/internal/proxy"
	"github.com/zachlagden/openhere/internal/sitefiles"
)

const (
	indexFile    = "index.html"
	notFoundFile = "404.html"
)

type serveTarget struct {
	prefix  string
	private bool
}

type siteServer struct {
	files    *sitefiles.Store
	index    fileIndex
	logger   *slog.Logger
	views    ViewRecorder
	proxies  proxy.Trust
	timeouts httptimeouts.Timeouts
}

func (s siteServer) serve(w http.ResponseWriter, r *http.Request, site Site, requestPath string, target serveTarget) {
	resolved, found, err := s.index.resolve(r.Context(), site.VersionID, requestPath)
	if err != nil {
		s.fail(w, err)
		return
	}
	if found && resolved.redirect {
		redirectToDirectory(w, r, target.prefix+requestPath+"/")
		return
	}
	if found {
		s.respondWithFile(w, r, fileResponse{site: site, rec: resolved.record, viewPath: requestPath, status: http.StatusOK, private: target.private})
		return
	}
	s.serveMissing(w, r, site, requestPath, target.private)
}

func (s siteServer) serveMissing(w http.ResponseWriter, r *http.Request, site Site, requestPath string, private bool) {
	if site.SPAMode && lacksExtension(requestPath) {
		if s.serveNamed(w, r, fileResponse{site: site, viewPath: analytics.PathRoot, status: http.StatusOK, private: private}, indexFile) {
			return
		}
	}
	if s.serveNamed(w, r, fileResponse{site: site, viewPath: analytics.PathNotFound, status: http.StatusNotFound, private: private}, notFoundFile) {
		return
	}
	writeNotFound(w, r)
}

func (s siteServer) serveNamed(w http.ResponseWriter, r *http.Request, resp fileResponse, name string) bool {
	rec, found, err := s.index.one(r.Context(), resp.site.VersionID, name)
	if err != nil {
		s.fail(w, err)
		return true
	}
	if !found {
		return false
	}
	resp.rec = rec
	s.respondWithFile(w, r, resp)
	return true
}

type fileResponse struct {
	site     Site
	rec      fileRecord
	viewPath string
	status   int
	private  bool
}

func (s siteServer) respondWithFile(w http.ResponseWriter, r *http.Request, resp fileResponse) {
	site, rec := resp.site, resp.rec
	file, info, err := s.files.Open(site.ID, site.VersionID, rec.Path)
	if errors.Is(err, sitefiles.ErrNotFound) || errors.Is(err, sitefiles.ErrNotRegular) {
		if errors.Is(err, sitefiles.ErrNotRegular) {
			s.logger.Warn("refused to serve a file that is not a regular file", "site_id", site.ID, "path", rec.Path)
		}
		writeNotFound(w, r)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	defer s.closeFile(file)
	contentType := contentTypeFor(rec)
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", cacheControlFor(contentType, resp.private))
	s.timeouts.ExtendDownload(w, info.Size())
	recorded := &statusRecorder{ResponseWriter: w}
	if resp.status == http.StatusOK {
		h.Set("ETag", `"`+rec.SHA256+`"`)
		http.ServeContent(recorded, r, "", info.ModTime(), file)
	} else {
		writeStatusWithBody(recorded, r, file, info.Size(), resp.status)
	}
	s.countView(r, site, resp.viewPath, contentType, recorded.status)
}

func writeStatusWithBody(w http.ResponseWriter, r *http.Request, file *os.File, size int64, status int) {
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, file); err != nil {
		slog.Debug("write response body", "error", err)
	}
}

func cacheControlFor(contentType string, private bool) string {
	switch {
	case private && isHTML(contentType):
		return cachePrivateHTML
	case private:
		return cachePrivateAsset
	case isHTML(contentType):
		return CacheHTML
	default:
		return CacheAsset
	}
}

func lacksExtension(requestPath string) bool {
	last := path.Base(requestPath)
	return last == "/" || path.Ext(last) == ""
}

func redirectToDirectory(w http.ResponseWriter, r *http.Request, target string) {
	location := url.URL{Path: target, RawQuery: r.URL.RawQuery}
	w.Header().Set("Location", location.String())
	writeText(w, http.StatusMovedPermanently, "Moved permanently\n", r.Method != http.MethodHead)
}

func (s siteServer) closeFile(file *os.File) {
	if err := file.Close(); err != nil {
		s.logger.Error("close site file", "error", err)
	}
}

func (s siteServer) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	s.logger.Error("serve site", "error", err)
	writeServerError(w)
}
