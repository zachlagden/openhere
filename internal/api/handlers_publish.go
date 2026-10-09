package api

import (
	"net/http"
	"time"

	"github.com/zachlagden/selfhost/internal/publish"
)

const workspaceHeader = "X-Workspace"

func (s *Server) registerPublishRoutes(mux *http.ServeMux) {
	versions := basePath + "/publish/{slug}/versions"
	mux.Handle("POST "+basePath+"/publish", s.protected(s.handlePublish))
	mux.Handle("PUT "+basePath+"/publish/{slug}", s.protected(s.handleRepublish))
	mux.Handle("PUT "+versions+"/{versionId}/files/{path...}", s.protected(s.handleUploadFile))
	mux.Handle("POST "+versions+"/{versionId}/finalize", s.protected(s.handleFinalize))
	mux.Handle("GET "+versions, s.protected(s.handleListVersions))
	mux.Handle("POST "+versions+"/{versionId}/restore", s.protected(s.handleRestore))
	mux.Handle("DELETE "+versions+"/{versionId}", s.protected(s.handleDeleteVersion))
	mux.Handle("GET "+basePath+"/sites/{slug}/files", s.protected(s.handleLiveManifest))
	mux.Handle("GET "+basePath+"/sites/{slug}/files/{path...}", s.protected(s.handleLiveFile))
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req publishRequest
	if !decodeJSONLimited(w, r, &req, maxManifestBodyBytes) {
		return
	}
	workspace := req.Workspace
	if workspace == "" {
		workspace = r.Header.Get(workspaceHeader)
	}
	pending, err := s.publish.Begin(r.Context(), publish.BeginInput{
		UserID: p.user.ID, Workspace: workspace, Slug: req.Slug, Folder: req.Folder, DisplayName: req.DisplayName,
		Description: req.Description, SpaMode: req.SpaMode, Files: toFileSpecs(req.Files),
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPublishResponse(pending))
}

func (s *Server) handleRepublish(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req republishRequest
	if !decodeJSONLimited(w, r, &req, maxManifestBodyBytes) {
		return
	}
	pending, err := s.publish.NewVersion(r.Context(), publish.NewVersionInput{
		UserID: p.user.ID, Site: r.PathValue("slug"), BaseVersionID: req.BaseVersionID, Files: toFileSpecs(req.Files),
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPublishResponse(pending))
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	cfg, err := s.settings.Get(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxFileBytes)
	spec, err := s.publish.Upload(r.Context(), p.user.ID, r.PathValue("slug"), r.PathValue("versionId"), r.PathValue("path"), r.Body)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, fileEnvelope{File: toFileResponse(spec)})
}

func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	result, err := s.publish.Finalize(r.Context(), p.user.ID, r.PathValue("slug"), r.PathValue("versionId"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resultResponse{Slug: result.Site.Slug, Version: toVersionResponse(result.Version)})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	versions, err := s.publish.List(r.Context(), p.user.ID, r.PathValue("slug"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	response := versionListResponse{Versions: make([]versionResponse, 0, len(versions))}
	for _, v := range versions {
		response.Versions = append(response.Versions, toVersionResponse(v))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	result, err := s.publish.Restore(r.Context(), p.user.ID, r.PathValue("slug"), r.PathValue("versionId"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resultResponse{Slug: result.Site.Slug, Version: toVersionResponse(result.Version)})
}

func (s *Server) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.publish.DeleteVersion(r.Context(), p.user.ID, r.PathValue("slug"), r.PathValue("versionId")); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLiveManifest(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	manifest, err := s.publish.LiveManifest(r.Context(), p.user.ID, r.PathValue("slug"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	response := manifestResponse{VersionID: optional(manifest.VersionID), Files: make([]fileResponse, 0, len(manifest.Files))}
	for _, f := range manifest.Files {
		response.Files = append(response.Files, toFileResponse(f))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleLiveFile(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	live, err := s.publish.OpenLiveFile(r.Context(), p.user.ID, r.PathValue("slug"), r.PathValue("path"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	defer func() {
		if err := live.File.Close(); err != nil {
			s.logger.Error("close live file", "error", err)
		}
	}()
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", "attachment")
	h.Set("ETag", `"`+live.Spec.SHA256+`"`)
	http.ServeContent(w, r, "", time.Time{}, live.File)
}
