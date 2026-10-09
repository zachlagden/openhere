package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"
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
	codeNotFound            = "not_found"
	codeKeyLimitReached     = "api_key_limit_reached"
	codeForbidden           = "forbidden"
	codeSlugTaken           = "slug_taken"
	codeFolderNameTaken     = "folder_name_taken"
	codeLastAdmin           = "last_admin"
	codePersonalWorkspace   = "personal_workspace"
	codeWorkspaceNotEmpty   = "workspace_not_empty"
	codeMemberExists        = "member_exists"
	codeUserNotFound        = "user_not_found"
	codeInternal            = "internal_error"
	codeAccountDisabled     = "account_disabled"
	codeVersionConflict     = "version_conflict"
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

func (s *Server) writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "Something went wrong.")
}
