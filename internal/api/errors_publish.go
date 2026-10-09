package api

import (
	"errors"
	"net/http"

	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/sitepath"
)

var publishFailures = []domainFailure{
	{publish.ErrTooManyFiles, http.StatusRequestEntityTooLarge, codeTooManyFiles, "The version has more files than this instance allows."},
	{publish.ErrVersionTooLarge, http.StatusRequestEntityTooLarge, codeVersionTooLarge, "The version is larger than this instance allows."},
	{publish.ErrFileTooLarge, http.StatusRequestEntityTooLarge, codeFileTooLarge, "A file is larger than this instance allows."},
	{publish.ErrSizeMismatch, http.StatusBadRequest, codeSizeMismatch, "The uploaded size does not match the manifest."},
	{publish.ErrHashMismatch, http.StatusBadRequest, codeHashMismatch, "The uploaded content does not match the manifest hash."},
	{publish.ErrVersionFinalized, http.StatusConflict, codeVersionFinalized, "The version is already finalized and cannot change."},
	{publish.ErrVersionExpired, http.StatusGone, codeVersionExpired, "The pending version expired. Start a new publish."},
	{publish.ErrVersionLive, http.StatusConflict, codeVersionLive, "The live version cannot be deleted. Restore another version first."},
	{publish.ErrNotFinalized, http.StatusConflict, codeNotFinalized, "Only finalized versions can be restored."},
	{publish.ErrFileNotInList, http.StatusNotFound, codeFileNotInManifest, "The path is not in this version's manifest."},
}

func writePublishFailure(w http.ResponseWriter, err error) bool {
	var conflict *publish.ConflictError
	if errors.As(err, &conflict) {
		current := optional(conflict.CurrentVersionID)
		writeJSON(w, http.StatusConflict, errorBody{
			Code: codeVersionConflict, Message: "The site's live version changed since the base version you sent.", CurrentVersionID: current,
		})
		return true
	}
	var missing *publish.MissingFilesError
	if errors.As(err, &missing) {
		writeJSON(w, http.StatusConflict, errorBody{
			Code: codeMissingFiles, Message: "Some files are missing or damaged. Upload them again, then finalize.", Missing: missing.Paths,
		})
		return true
	}
	if errors.Is(err, sitepath.ErrInvalid) {
		writeError(w, http.StatusBadRequest, codeInvalidPath, err.Error())
		return true
	}
	if isMaxBytes(err) {
		writeError(w, http.StatusRequestEntityTooLarge, codeFileTooLarge, "The upload is larger than this instance allows for one file.")
		return true
	}
	for _, failure := range publishFailures {
		if errors.Is(err, failure.target) {
			writeError(w, failure.status, failure.code, failure.message)
			return true
		}
	}
	return false
}

func isMaxBytes(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}
