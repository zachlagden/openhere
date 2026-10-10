package fsperm

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
)

const (
	DirMode  fs.FileMode = 0o700
	FileMode fs.FileMode = 0o600
)

var chmod = os.Chmod

func EnsureDir(path string, logger *slog.Logger) error {
	if err := os.MkdirAll(path, DirMode); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	Restrict(path, DirMode, logger)
	return nil
}

func Restrict(path string, mode fs.FileMode, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		logger.Warn("could not read permissions", "path", path, "error", err)
		return
	}
	if info.Mode().Perm() == mode {
		return
	}
	if err := chmod(path, mode); err != nil {
		logger.Warn("could not restrict permissions", "path", path, "mode", mode.String(), "error", err)
	}
}

func CreateFile(path string, logger *slog.Logger) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, FileMode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	Restrict(path, FileMode, logger)
	return nil
}
