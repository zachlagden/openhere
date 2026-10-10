package fsperm

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDirCreatesAndTightensDirectories(t *testing.T) {
	root := t.TempDir()
	created := filepath.Join(root, "data")
	if err := EnsureDir(created, nil); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(root, "loose")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(existing, nil); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{created, existing} {
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != DirMode {
			t.Errorf("%s mode = %v, err %v; want %v", dir, info.Mode().Perm(), err, DirMode)
		}
	}
}

func TestCreateFileMakesAPrivateFileAndFixesAnOpenOne(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.db")
	if err := CreateFile(fresh, nil); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(dir, "loose.db")
	if err := os.WriteFile(loose, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateFile(loose, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fresh, loose} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != FileMode {
			t.Errorf("%s mode = %v, err %v; want %v", path, info.Mode().Perm(), err, FileMode)
		}
	}
	content, err := os.ReadFile(loose)
	if err != nil || string(content) != "data" {
		t.Errorf("CreateFile changed the contents: %q, %v", content, err)
	}
}

func TestRestrictWarnsWhenItCannotChangeAMode(t *testing.T) {
	original := chmod
	chmod = func(string, os.FileMode) error { return errors.New("operation not permitted") }
	t.Cleanup(func() { chmod = original })
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	Restrict(path, FileMode, logger)
	if !strings.Contains(logs.String(), "could not restrict permissions") || !strings.Contains(logs.String(), path) {
		t.Errorf("log = %q", logs.String())
	}
}

func TestRestrictIgnoresMissingPaths(t *testing.T) {
	var logs bytes.Buffer
	Restrict(filepath.Join(t.TempDir(), "absent"), FileMode, slog.New(slog.NewTextHandler(&logs, nil)))
	if logs.Len() != 0 {
		t.Errorf("log = %q", logs.String())
	}
}
