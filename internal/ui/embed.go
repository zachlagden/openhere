package ui

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
)

//go:embed all:bundle
var embedded embed.FS

func Dist() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "bundle")
	if err != nil {
		return nil, fmt.Errorf("open embedded dashboard: %w", err)
	}
	return sub, nil
}

func bytesReader(body []byte) io.ReadSeeker {
	return bytes.NewReader(body)
}

func writeBody(w io.Writer, body []byte) {
	if _, err := w.Write(body); err != nil {
		slog.Debug("write dashboard response", "error", err)
	}
}
