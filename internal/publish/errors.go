package publish

import (
	"errors"
	"strings"
)

var (
	ErrTooManyFiles     = errors.New("the version has more files than the instance allows")
	ErrVersionTooLarge  = errors.New("the version is larger than the instance allows")
	ErrFileTooLarge     = errors.New("a file is larger than the instance allows")
	ErrSizeMismatch     = errors.New("the uploaded size does not match the manifest")
	ErrHashMismatch     = errors.New("the uploaded content does not match the manifest hash")
	ErrVersionFinalized = errors.New("the version is already finalized")
	ErrVersionExpired   = errors.New("the pending version has expired")
	ErrVersionLive      = errors.New("the live version cannot be deleted")
	ErrNotFinalized     = errors.New("the version is not finalized")
	ErrFileNotInList    = errors.New("the path is not in the version manifest")

	errNotPending = errors.New("version is not pending")
)

type ConflictError struct {
	CurrentVersionID string
}

func (e *ConflictError) Error() string {
	return "the site's live version is " + displayID(e.CurrentVersionID)
}

func displayID(id string) string {
	if id == "" {
		return "none"
	}
	return id
}

type MissingFilesError struct {
	Paths []string
}

func (e *MissingFilesError) Error() string {
	return "files missing or damaged: " + strings.Join(e.Paths, ", ")
}
