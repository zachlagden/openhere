package hosting

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/zachlagden/selfhost/internal/sitefiles"
)

const (
	indexFile    = "index.html"
	notFoundFile = "404.html"
)

type siteServer struct {
	files  *sitefiles.Store
	index  fileIndex
	logger *slog.Logger
}

func (s siteServer) serve(w http.ResponseWriter, r *http.Request, site Site, requestPath, prefix string) {
	resolved, found, err := s.index.resolve(r.Context(), site.VersionID, requestPath)
	if err != nil {
		s.fail(w, err)
		return
	}
	if found && resolved.redirect {
		redirectToDirectory(w, r, prefix+requestPath+"/")
		return
	}
	if found {
		s.respondWithFile(w, r, site, resolved.record, http.StatusOK)
		return
	}
	s.serveMissing(w, r, site, requestPath)
}

func (s siteServer) serveMissing(w http.ResponseWriter, r *http.Request, site Site, requestPath string) {
	if site.SPAMode && lacksExtension(requestPath) {
		if s.serveNamed(w, r, site, indexFile, http.StatusOK) {
			return
		}
	}
	if s.serveNamed(w, r, site, notFoundFile, http.StatusNotFound) {
		return
	}
	writeNotFound(w, r)
}

func (s siteServer) serveNamed(w http.ResponseWriter, r *http.Request, site Site, name string, status int) bool {
	rec, found, err := s.index.one(r.Context(), site.VersionID, name)
	if err != nil {
		s.fail(w, err)
		return true
	}
	if !found {
		return false
	}
	s.respondWithFile(w, r, site, rec, status)
	return true
}

func (s siteServer) respondWithFile(w http.ResponseWriter, r *http.Request, site Site, rec fileRecord, status int) {
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
	h.Set("Cache-Control", cacheControlFor(contentType))
	if status == http.StatusOK {
		h.Set("ETag", `"`+rec.SHA256+`"`)
		http.ServeContent(w, r, "", info.ModTime(), file)
		return
	}
	writeStatusWithBody(w, r, file, info.Size(), status)
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

func cacheControlFor(contentType string) string {
	if isHTML(contentType) {
		return CacheHTML
	}
	return CacheAsset
}

func lacksExtension(requestPath string) bool {
	last := path.Base(requestPath)
	return last == "/" || path.Ext(last) == ""
}

func redirectToDirectory(w http.ResponseWriter, r *http.Request, target string) {
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
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
