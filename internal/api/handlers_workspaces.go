package api

import (
	"net/http"
)

type createWorkspaceRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type renameWorkspaceRequest struct {
	Name string `json:"name"`
}

func (s *Server) registerWorkspaceRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+basePath+"/workspaces", s.protected(s.handleListWorkspaces))
	mux.Handle("POST "+basePath+"/workspaces", s.protected(s.handleCreateWorkspace))
	mux.Handle("GET "+basePath+"/workspaces/{workspace}", s.protected(s.handleGetWorkspace))
	mux.Handle("PATCH "+basePath+"/workspaces/{workspace}", s.protected(s.handleRenameWorkspace))
	mux.Handle("DELETE "+basePath+"/workspaces/{workspace}", s.protected(s.handleDeleteWorkspace))
	mux.Handle("GET "+basePath+"/workspaces/{workspace}/members", s.protected(s.handleListMembers))
	mux.Handle("POST "+basePath+"/workspaces/{workspace}/members", s.protected(s.handleAddMember))
	mux.Handle("PATCH "+basePath+"/workspaces/{workspace}/members/{memberId}", s.protected(s.handleUpdateMember))
	mux.Handle("DELETE "+basePath+"/workspaces/{workspace}/members/{memberId}", s.protected(s.handleRemoveMember))
	mux.Handle("GET "+basePath+"/workspaces/{workspace}/folders", s.protected(s.handleListFolders))
	mux.Handle("POST "+basePath+"/workspaces/{workspace}/folders", s.protected(s.handleCreateFolder))
	mux.Handle("PATCH "+basePath+"/workspaces/{workspace}/folders/{folder}", s.protected(s.handleRenameFolder))
	mux.Handle("DELETE "+basePath+"/workspaces/{workspace}/folders/{folder}", s.protected(s.handleDeleteFolder))
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	list, err := s.workspaces.List(r.Context(), p.user.ID)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	response := workspaceListResponse{Workspaces: make([]workspaceResponse, 0, len(list))}
	for _, acc := range list {
		response.Workspaces = append(response.Workspaces, toWorkspaceResponse(acc))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req createWorkspaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	acc, err := s.workspaces.CreateShared(r.Context(), p.user.ID, req.Name, req.Slug)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, workspaceEnvelope{Workspace: toWorkspaceResponse(acc)})
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	acc, err := s.workspaces.Access(r.Context(), p.user.ID, r.PathValue("workspace"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, workspaceEnvelope{Workspace: toWorkspaceResponse(acc)})
}

func (s *Server) handleRenameWorkspace(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req renameWorkspaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	acc, err := s.workspaces.Rename(r.Context(), p.user.ID, r.PathValue("workspace"), req.Name)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, workspaceEnvelope{Workspace: toWorkspaceResponse(acc)})
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.workspaces.Delete(r.Context(), p.user.ID, r.PathValue("workspace")); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
