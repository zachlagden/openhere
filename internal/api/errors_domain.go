package api

import (
	"errors"
	"net/http"

	"github.com/zachlagden/selfhost/internal/agentcode"
	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/invites"
	"github.com/zachlagden/selfhost/internal/passwordreset"
	"github.com/zachlagden/selfhost/internal/policy"
	"github.com/zachlagden/selfhost/internal/settings"
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
	{sites.ErrFolderHasGrants, http.StatusConflict, codeFolderHasGrants, "People still have access to this folder. Remove their access first, or delete the folder anyway."},
	{sites.ErrFolderNameTaken, http.StatusConflict, codeFolderNameTaken, "A folder with that name already exists in this workspace."},
	{workspaces.ErrLastAdmin, http.StatusConflict, codeLastAdmin, "A shared workspace needs at least one workspace-wide admin."},
	{workspaces.ErrPersonalWorkspace, http.StatusConflict, codePersonalWorkspace, "A personal workspace cannot be deleted, shared or have its members changed."},
	{workspaces.ErrNotEmpty, http.StatusConflict, codeWorkspaceNotEmpty, "Delete or move the workspace's sites first."},
	{workspaces.ErrMemberExists, http.StatusConflict, codeMemberExists, "That user already has this access."},
	{settings.ErrWildcardNotVerified, http.StatusConflict, codeWildcardNotVerified, "Subdomain mode needs a successful wildcard check. Run POST /admin/hosting/check first."},
	{settings.ErrNoHostingMode, http.StatusConflict, codeNoHostingMode, "At least one hosting mode must stay on. Switch the other mode on in the same change."},
	{settings.ErrBaseDomainInUse, http.StatusConflict, codeBaseDomainInUse, "A custom domain already uses that base domain or a name under it. Delete the domain first."},
	{domains.ErrNotFound, http.StatusNotFound, codeNotFound, "Not found."},
	{domains.ErrTaken, http.StatusConflict, codeDomainTaken, "That domain name is already in use."},
	{domains.ErrLimitReached, http.StatusConflict, codeDomainLimitReached, "This site has reached its limit of custom domains. Delete one first."},
	{domains.ErrNotVerified, http.StatusConflict, codeDomainNotVerified, "Verify the domain before making it the primary domain."},
	{policy.ErrSharedCreationRestricted, http.StatusForbidden, codeForbidden, "Only the root user can create workspaces on this instance."},
	{invites.ErrInviteInvalid, http.StatusNotFound, codeNotFound, "Not found."},
	{invites.ErrInviteNotFound, http.StatusNotFound, codeNotFound, "Not found."},
	{invites.ErrNotPermitted, http.StatusForbidden, codeForbidden, "Your account is not allowed to send invitations here."},
	{invites.ErrAccepted, http.StatusConflict, codeInviteAccepted, "That invite has already been accepted."},
	{invites.ErrEmailRegistered, http.StatusConflict, codeEmailRegistered, "That email address already has an account. Add them to the workspace from its members page instead."},
	{passwordreset.ErrInvalidToken, http.StatusBadRequest, codeInvalidResetToken, "This reset link is invalid or has expired."},
	{agentcode.ErrEmailNotConfigured, http.StatusConflict, codeEmailNotConfigured, agentNoEmailMessage},
	{agentcode.ErrCodeInvalid, http.StatusBadRequest, codeInvalidCode, "The code is wrong, expired or already used. Request a new one."},
	{auth.ErrAPIKeyLimit, http.StatusConflict, codeKeyLimitReached, "The account has reached the limit of 50 active API keys. Revoke one first."},
	{auth.ErrUserNotFound, http.StatusNotFound, codeNotFound, "Not found."},
	{auth.ErrSelfDisable, http.StatusConflict, codeCannotDisableSelf, "You cannot disable your own account."},
	{auth.ErrLastRoot, http.StatusConflict, codeLastRoot, "The last root user cannot be disabled."},
	{auth.ErrUserDisabled, http.StatusConflict, codeUserDisabled, "That account is disabled. Enable it first."},
	{workspaces.ErrUserNotFound, http.StatusNotFound, codeUserNotFound, "No user has that email address."},
}

func (s *Server) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *validate.Error
	if errors.As(err, &invalid) {
		writeError(w, http.StatusBadRequest, codeValidationFailed, invalid.Message)
		return
	}
	var authInvalid *auth.ValidationError
	if errors.As(err, &authInvalid) {
		writeError(w, http.StatusBadRequest, codeValidationFailed, authInvalid.Message)
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
