package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxBodyBytes = 16 << 10

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, codeUnsupportedMedia, "Content-Type must be application/json.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeDecodeError(w, err)
		return false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			writeError(w, http.StatusBadRequest, codeInvalidJSON, "Request body must contain a single JSON value.")
			return false
		}
		writeDecodeError(w, err)
		return false
	}
	return true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, codeBodyTooLarge, "Request body is too large.")
		return
	}
	writeError(w, http.StatusBadRequest, codeInvalidJSON, "Request body is not valid JSON for this endpoint.")
}
