package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/zachlagden/openhere/internal/hashlimit"
)

const (
	codeInvalidJSON         = "invalid_json"
	codeValidationFailed    = "validation_failed"
	codeUnsupportedMedia    = "unsupported_media_type"
	codeBodyTooLarge        = "body_too_large"
	codeUnauthenticated     = "unauthenticated"
	codeInvalidLogin        = "invalid_credentials"
	codeInvalidSetupToken   = "invalid_setup_token"
	codeSetupComplete       = "setup_complete"
	codeCSRFRejected        = "csrf_rejected"
	codeRateLimited         = "rate_limited"
	codeServerBusy          = "server_busy"
	codeNotFound            = "not_found"
	codeKeyLimitReached     = "api_key_limit_reached"
	codeForbidden           = "forbidden"
	codeSlugTaken           = "slug_taken"
	codeFolderNameTaken     = "folder_name_taken"
	codeFolderHasGrants     = "folder_has_grants"
	codeLastAdmin           = "last_admin"
	codePersonalWorkspace   = "personal_workspace"
	codeWorkspaceNotEmpty   = "workspace_not_empty"
	codeMemberExists        = "member_exists"
	codeUserNotFound        = "user_not_found"
	codeInternal            = "internal_error"
	codeAccountDisabled     = "account_disabled"
	codeCannotDisableSelf   = "cannot_disable_self"
	codeLastRoot            = "last_root"
	codeUserDisabled        = "user_disabled"
	codeInviteAccepted      = "invite_accepted"
	codeEmailRegistered     = "email_registered"
	codeInvalidResetToken   = "invalid_reset_token"
	codeInvalidCode         = "invalid_code"
	codeVersionConflict     = "version_conflict"
	codeTooManyPending      = "too_many_pending"
	codeTooManyFiles        = "too_many_files"
	codeVersionTooLarge     = "version_too_large"
	codeFileTooLarge        = "file_too_large"
	codeHashMismatch        = "hash_mismatch"
	codeSizeMismatch        = "size_mismatch"
	codeMissingFiles        = "missing_files"
	codeInvalidPath         = "invalid_path"
	codeVersionFinalized    = "version_finalized"
	codeVersionExpired      = "version_expired"
	codeVersionLive         = "version_live"
	codeNotFinalized        = "version_not_finalized"
	codeFileNotInManifest   = "file_not_in_manifest"
	codeWildcardNotVerified = "wildcard_not_verified"
	codeNoHostingMode       = "no_hosting_mode"
	codeBaseDomainInUse     = "base_domain_in_use"
	codeDomainTaken         = "domain_taken"
	codeDomainLimitReached  = "domain_limit_reached"
	codeDomainNotVerified   = "domain_not_verified"
	codeGeoNotConfigured    = "geo_not_configured"
)

type errorBody struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RetryAfter *int   `json:"retry_after,omitempty"`

	CurrentVersionID *string  `json:"currentVersionId,omitempty"`
	Missing          []string `json:"missing,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Debug("write response body", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Code: code, Message: message})
}

func writeRateLimited(w http.ResponseWriter, wait time.Duration) {
	seconds := int(math.Ceil(wait.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, errorBody{
		Code:       codeRateLimited,
		Message:    "Too many attempts. Try again later.",
		RetryAfter: &seconds,
	})
}

func writeServerBusy(w http.ResponseWriter) {
	seconds := int(math.Ceil(hashlimit.Default().RetryAfter().Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusServiceUnavailable, errorBody{
		Code:       codeServerBusy,
		Message:    "The server is busy. Try again in a moment.",
		RetryAfter: &seconds,
	})
}

func routeLabel(r *http.Request) string {
	if r.Pattern == "" {
		return "unmatched"
	}
	return r.Pattern
}

func (s *Server) writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, hashlimit.ErrBusy) {
		writeServerBusy(w)
		return
	}
	s.logger.Error("request failed", "method", r.Method, "route", routeLabel(r), "error", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "Something went wrong.")
}
