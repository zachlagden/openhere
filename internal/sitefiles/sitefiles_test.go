package sitefiles

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/sitepath"
)

const (
	siteID    = "0123456789abcdef0123456789abcdef"
	versionID = "fedcba9876543210fedcba9876543210"
)

func sum(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func put(t *testing.T, s *Store, rel, content string) error {
	t.Helper()
	return s.Put(siteID, versionID, rel, strings.NewReader(content), int64(len(content)), sum(content))
}

func TestPutAndOpen(t *testing.T) {
	s := newStore(t)
	if err := put(t, s, "assets/app.js", "console.log(1)"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	file, info, err := s.Open(siteID, versionID, "assets/app.js")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if string(body) != "console.log(1)" || info.Size() != int64(len(body)) {
		t.Errorf("content = %q, size = %d", body, info.Size())
	}
	want := filepath.Join(s.root, "sites", siteID, "versions", versionID, "files", "assets", "app.js")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("file is not at the documented location: %v", err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Errorf("stored file is writable: %v", info.Mode())
	}
}

func TestPutEmptyFile(t *testing.T) {
	s := newStore(t)
	if err := put(t, s, "empty.txt", ""); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Verify(siteID, versionID, Expected{Path: "empty.txt", Size: 0, SHA256: sum("")}); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestPutRejectsMismatches(t *testing.T) {
	s := newStore(t)
	err := s.Put(siteID, versionID, "a.txt", strings.NewReader("hello"), 5, sum("other"))
	if !errors.Is(err, ErrHashMismatch) {
		t.Errorf("wrong hash: error = %v, want ErrHashMismatch", err)
	}
	err = s.Put(siteID, versionID, "a.txt", strings.NewReader("hello world"), 5, sum("hello"))
	if !errors.Is(err, ErrSizeMismatch) {
		t.Errorf("body longer than declared: error = %v, want ErrSizeMismatch", err)
	}
	err = s.Put(siteID, versionID, "a.txt", strings.NewReader("hi"), 5, sum("hello"))
	if !errors.Is(err, ErrSizeMismatch) {
		t.Errorf("body shorter than declared: error = %v, want ErrSizeMismatch", err)
	}
	if _, _, err := s.Open(siteID, versionID, "a.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rejected upload left a file behind: %v", err)
	}
	assertNoTemporaryFiles(t, s)
}

func assertNoTemporaryFiles(t *testing.T, s *Store) {
	t.Helper()
	entries, err := os.ReadDir(s.tmpDir())
	if err != nil {
		t.Fatalf("read tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%d temporary files left behind", len(entries))
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestPutCleansUpAfterReadError(t *testing.T) {
	s := newStore(t)
	if err := s.Put(siteID, versionID, "a.txt", failingReader{}, 5, sum("hello")); err == nil {
		t.Error("read error was swallowed")
	}
	assertNoTemporaryFiles(t, s)
}

func TestPutRejectsUnsafePaths(t *testing.T) {
	s := newStore(t)
	for _, rel := range []string{"../escape.txt", "/abs.txt", "a/../../b", `a\b`, "a//b", ""} {
		err := put(t, s, rel, "x")
		if !errors.Is(err, sitepath.ErrInvalid) {
			t.Errorf("Put(%q) = %v, want ErrInvalid", rel, err)
		}
	}
	assertNoTemporaryFiles(t, s)
	if _, err := os.Stat(filepath.Join(s.root, "escape.txt")); err == nil {
		t.Error("a file escaped the files directory")
	}
}

func TestPutRejectsUnsafeIDs(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"..", "../x", "short", strings.Repeat("A", 32), ""} {
		if err := s.Put(id, versionID, "a.txt", strings.NewReader("x"), 1, sum("x")); !errors.Is(err, ErrUnsafeID) {
			t.Errorf("site id %q: error = %v, want ErrUnsafeID", id, err)
		}
		if err := s.Put(siteID, id, "a.txt", strings.NewReader("x"), 1, sum("x")); !errors.Is(err, ErrUnsafeID) {
			t.Errorf("version id %q: error = %v, want ErrUnsafeID", id, err)
		}
		if err := s.RemoveSite(id); !errors.Is(err, ErrUnsafeID) {
			t.Errorf("RemoveSite(%q) = %v, want ErrUnsafeID", id, err)
		}
	}
}

func TestPutNeverFollowsSymlinks(t *testing.T) {
	s := newStore(t)
	if err := put(t, s, "keep.txt", "keep"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	filesDir, err := s.filesDir(siteID, versionID)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(filesDir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "inside.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(siteID, versionID, "link/inside.txt"); !errors.Is(err, ErrNotRegular) {
		t.Errorf("Open through a symlinked directory = %v, want ErrNotRegular", err)
	}
	if _, _, err := s.Open(siteID, versionID, "missing/inside.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open below a missing directory = %v, want ErrNotFound", err)
	}
	if err := put(t, s, "link/stolen.txt", "x"); err == nil {
		t.Error("Put wrote through a symlinked directory")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("a file landed outside the files directory: %v", entries)
	}

	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(filesDir, "leak.txt")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(siteID, versionID, "leak.txt"); !errors.Is(err, ErrNotRegular) {
		t.Errorf("Open of a symlink = %v, want ErrNotRegular", err)
	}
	if err := s.Verify(siteID, versionID, Expected{Path: "leak.txt", Size: 6, SHA256: sum("secret")}); err == nil {
		t.Error("Verify accepted a symlink")
	}
	if err := put(t, s, "leak.txt", "replacement"); err == nil {
		t.Error("Put replaced a symlink")
	}
	if body, err := os.ReadFile(filepath.Clean(secret)); err != nil || string(body) != "secret" {
		t.Errorf("symlink target changed: %q, %v", body, err)
	}
}

func TestPutRefusesFileDirectoryConflict(t *testing.T) {
	s := newStore(t)
	if err := put(t, s, "docs", "file"); err != nil {
		t.Fatal(err)
	}
	if err := put(t, s, "docs/index.html", "x"); err == nil {
		t.Error("Put created a directory where a file exists")
	}
	if err := put(t, s, "other/index.html", "x"); err != nil {
		t.Fatal(err)
	}
	if err := put(t, s, "other", "x"); err == nil {
		t.Error("Put replaced a directory with a file")
	}
}

func TestStageThenCommitAndDiscard(t *testing.T) {
	s := newStore(t)
	staged, err := s.Stage(strings.NewReader("abc"), 3, sum("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Discard(); err != nil {
		t.Fatal(err)
	}
	assertNoTemporaryFiles(t, s)
	staged, err = s.Stage(strings.NewReader("abc"), 3, sum("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Commit(s, siteID, versionID, "../bad"); err == nil {
		t.Error("Commit accepted an unsafe path")
	}
	assertNoTemporaryFiles(t, s)
}

func TestVerify(t *testing.T) {
	s := newStore(t)
	if err := put(t, s, "a.txt", "hello"); err != nil {
		t.Fatal(err)
	}
	good := Expected{Path: "a.txt", Size: 5, SHA256: sum("hello")}
	if err := s.Verify(siteID, versionID, good); err != nil {
		t.Errorf("Verify: %v", err)
	}
	tests := map[string]struct {
		want Expected
		err  error
	}{
		"missing":    {Expected{Path: "b.txt", Size: 5, SHA256: sum("hello")}, ErrNotFound},
		"wrong size": {Expected{Path: "a.txt", Size: 6, SHA256: sum("hello")}, ErrSizeMismatch},
		"wrong hash": {Expected{Path: "a.txt", Size: 5, SHA256: sum("world")}, ErrHashMismatch},
	}
	for name, tc := range tests {
		if err := s.Verify(siteID, versionID, tc.want); !errors.Is(err, tc.err) {
			t.Errorf("%s: error = %v, want %v", name, err, tc.err)
		}
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	s := newStore(t)
	if err := put(t, s, "a.txt", "hello"); err != nil {
		t.Fatal(err)
	}
	filesDir, _ := s.filesDir(siteID, versionID)
	target := filepath.Join(filesDir, "a.txt")
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("jello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(siteID, versionID, Expected{Path: "a.txt", Size: 5, SHA256: sum("hello")}); !errors.Is(err, ErrHashMismatch) {
		t.Errorf("error = %v, want ErrHashMismatch", err)
	}
}

func TestRemoveAndList(t *testing.T) {
	s := newStore(t)
	otherVersion := strings.Repeat("a", 32)
	if err := put(t, s, "a.txt", "x"); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(siteID, otherVersion, "b.txt", bytes.NewReader([]byte("y")), 1, sum("y")); err != nil {
		t.Fatal(err)
	}
	sitesFound, err := s.SiteDirs()
	if err != nil || len(sitesFound) != 1 || sitesFound[0] != siteID {
		t.Errorf("SiteDirs = %v, %v", sitesFound, err)
	}
	versions, err := s.VersionDirs(siteID)
	if err != nil || len(versions) != 2 {
		t.Errorf("VersionDirs = %v, %v", versions, err)
	}
	if err := s.RemoveVersion(siteID, versionID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(siteID, versionID, "a.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed version still opens: %v", err)
	}
	if _, _, err := s.Open(siteID, otherVersion, "b.txt"); err != nil {
		t.Errorf("other version damaged: %v", err)
	}
	if err := s.RemoveSite(siteID); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveSite(siteID); err != nil {
		t.Errorf("removing a missing site failed: %v", err)
	}
	if left, _ := s.SiteDirs(); len(left) != 0 {
		t.Errorf("site directories left: %v", left)
	}
	if versions, err := s.VersionDirs(siteID); err != nil || len(versions) != 0 {
		t.Errorf("VersionDirs for a removed site = %v, %v", versions, err)
	}
}

func TestSweepTemporary(t *testing.T) {
	s := newStore(t)
	old := filepath.Join(s.tmpDir(), "upload-old")
	fresh := filepath.Join(s.tmpDir(), "upload-fresh")
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	removed, err := s.SweepTemporary(time.Now().Add(-24 * time.Hour))
	if err != nil || removed != 1 {
		t.Fatalf("SweepTemporary = %d, %v", removed, err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("old temporary file survived")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("fresh temporary file was removed")
	}
}

func TestStoreDirectoriesAreOwnerOnlyAndExistingOnesAreTightened(t *testing.T) {
	root := t.TempDir()
	loose := filepath.Join(root, "sites")
	if err := os.Mkdir(loose, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sites", "tmp"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %v, err %v; want 0700", name, info.Mode().Perm(), err)
		}
	}
}
