package api

import (
	"errors"
	"net/http"

	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type domainFailure struct {
	target  error
	status  int
	code    string
	message string
}

var domainFailures = []domainFailure{
	{workspaces.ErrNotFound, http.StatusNotFound, codeNotFound, "Not found."},
	{workspaces.ErrForbidden, http.StatusForbidden, codeForbidden, "Your role does not allow this."},
	{workspaces.ErrSlugTaken, http.StatusConflict, codeSlugTaken, "That slug is already taken."},
	{sites.ErrSlugTaken, http.StatusConflict, codeSlugTaken, "That slug is already taken."},
	{sites.ErrFolderNameTaken, http.StatusConflict, codeFolderNameTaken, "A folder with that name already exists in this workspace."},
	{workspaces.ErrLastAdmin, http.StatusConflict, codeLastAdmin, "A shared workspace needs at least one workspace-wide admin."},
	{workspaces.ErrPersonalWorkspace, http.StatusConflict, codePersonalWorkspace, "A personal workspace cannot be deleted, shared or have its members changed."},
	{workspaces.ErrNotEmpty, http.StatusConflict, codeWorkspaceNotEmpty, "Delete or move the workspace's sites first."},
	{workspaces.ErrMemberExists, http.StatusConflict, codeMemberExists, "That user already has this access."},
	{workspaces.ErrUserNotFound, http.StatusNotFound, codeUserNotFound, "No user has that email address."},
}

func (s *Server) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *validate.Error
	if errors.As(err, &invalid) {
		writeError(w, http.StatusBadRequest, codeValidationFailed, invalid.Message)
		return
	}
	if writePublishFailure(w, err) {
		return
	}
	for _, failure := range domainFailures {
		if errors.Is(err, failure.target) {
			writeError(w, failure.status, failure.code, failure.message)
			return
		}
	}
	s.writeInternal(w, r, err)
}
