package api

import "net/http"

type folderRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	folders, err := s.sites.ListFolders(r.Context(), p.user.ID, r.PathValue("workspace"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	response := folderListResponse{Folders: make([]folderResponse, 0, len(folders))}
	for _, f := range folders {
		response.Folders = append(response.Folders, toFolderResponse(f))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req folderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	folder, err := s.sites.CreateFolder(r.Context(), p.user.ID, r.PathValue("workspace"), req.Name)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, folderEnvelope{Folder: toFolderResponse(folder)})
}

func (s *Server) handleRenameFolder(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req folderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	folder, err := s.sites.RenameFolder(r.Context(), p.user.ID, r.PathValue("workspace"), r.PathValue("folder"), req.Name)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, folderEnvelope{Folder: toFolderResponse(folder)})
}

func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.sites.DeleteFolder(r.Context(), p.user.ID, r.PathValue("workspace"), r.PathValue("folder"), r.URL.Query().Get("force") == "true"); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
