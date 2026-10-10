package store

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func withUmask(t *testing.T, mask int) {
	t.Helper()
	previous := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(previous) })
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("stat %s: %v", path, err)
		return
	}
	if info.Mode().Perm() != want {
		t.Errorf("%s mode = %v, want %v", path, info.Mode().Perm(), want)
	}
}

func TestSQLiteFilesAreOwnerOnlyEvenWithAPermissiveUmask(t *testing.T) {
	withUmask(t, 0)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "data")
	s, err := Open(ctx, Options{DataDir: dir})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := s.ExecContext(ctx, "INSERT INTO instance_settings (key, value, updated_at) VALUES ('probe', 'x', $1)", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o700)
	files, err := filepath.Glob(filepath.Join(dir, sqliteFileName+"*"))
	if err != nil || len(files) < 3 {
		t.Fatalf("database files = %v, err %v; want the database, -wal and -shm", files, err)
	}
	for _, file := range files {
		assertMode(t, file, 0o600)
	}
}

func TestExistingLoosePermissionsAreTightenedAtStart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(dir, sqliteFileName)
	if err := os.WriteFile(database, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(database, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(database+"-wal", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(database+"-wal", 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, Options{DataDir: dir})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	assertMode(t, dir, 0o700)
	assertMode(t, database, 0o600)
	assertMode(t, database+"-wal", 0o600)
}
