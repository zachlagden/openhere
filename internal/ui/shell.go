package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	indexName       = "index.html"
	assetsPrefix    = "assets/"
	notBuiltText    = "The dashboard is not built. Run \"pnpm --dir web build\" and rebuild the server.\n"
	notFoundBody    = "{\"code\":\"not_found\",\"message\":\"Not found.\"}\n"
	contentSecurity = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; " +
		"connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
)

var extraTypes = map[string]string{
	".woff2":       "font/woff2",
	".woff":        "font/woff",
	".ico":         "image/x-icon",
	".txt":         "text/plain; charset=utf-8",
	".webmanifest": "application/manifest+json",
	".map":         "application/json",
}

type asset struct {
	body        []byte
	contentType string
	etag        string
	immutable   bool
}

type Shell struct {
	assets map[string]asset
	index  *asset
}

func New(files fs.FS) (*Shell, error) {
	shell := &Shell{assets: map[string]asset{}}
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if hidden(name) {
			return nil
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		shell.assets[name] = newAsset(name, body)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load dashboard files: %w", err)
	}
	if index, ok := shell.assets[indexName]; ok {
		shell.index = &index
		delete(shell.assets, indexName)
	}
	return shell, nil
}

func hidden(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func newAsset(name string, body []byte) asset {
	sum := sha256.Sum256(body)
	return asset{
		body:        body,
		contentType: contentTypeFor(name, body),
		etag:        "\"" + hex.EncodeToString(sum[:16]) + "\"",
		immutable:   strings.HasPrefix(name, assetsPrefix),
	}
}

func contentTypeFor(name string, body []byte) string {
	ext := strings.ToLower(path.Ext(name))
	if known, ok := extraTypes[ext]; ok {
		return known
	}
	if detected := mime.TypeByExtension(ext); detected != "" {
		return detected
	}
	return http.DetectContentType(body)
}

func (s *Shell) Built() bool {
	return s.index != nil
}

func (s *Shell) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !servesShell(r) {
			next.ServeHTTP(w, r)
			return
		}
		s.serve(w, r)
	})
}

func servesShell(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	p := r.URL.Path
	if p == "/api" || p == "/s" || strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/s/") || strings.HasPrefix(p, "/.well-known/") {
		return false
	}
	cleaned := path.Clean(p)
	if cleaned == "/" {
		return p == "/"
	}
	return p == cleaned || p == cleaned+"/"
}

func (s *Shell) serve(w http.ResponseWriter, r *http.Request) {
	name := strings.Trim(r.URL.Path, "/")
	if found, ok := s.assets[name]; ok {
		serveAsset(w, r, found)
		return
	}
	if path.Ext(name) != "" && name != indexName {
		writeNotFound(w)
		return
	}
	s.serveShell(w, r)
}

func (s *Shell) serveShell(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	if s.index == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		if r.Method != http.MethodHead {
			writeBody(w, []byte(notBuiltText))
		}
		return
	}
	serveAsset(w, r, *s.index)
}

func serveAsset(w http.ResponseWriter, r *http.Request, a asset) {
	setSecurityHeaders(w)
	h := w.Header()
	h.Set("Content-Type", a.contentType)
	h.Set("ETag", a.etag)
	if a.immutable {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, "", time.Time{}, bytesReader(a.body))
}

func writeNotFound(w http.ResponseWriter) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	writeBody(w, []byte(notFoundBody))
}

func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", contentSecurity)
}
