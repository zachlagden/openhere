package sitefiles

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zachlagden/selfhost/internal/sitepath"
)

type Staged struct {
	path string
}

func (s *Store) Put(siteID, versionID, rel string, body io.Reader, size int64, sha256Hex string) error {
	staged, err := s.Stage(body, size, sha256Hex)
	if err != nil {
		return err
	}
	return staged.Commit(s, siteID, versionID, rel)
}

func (s *Store) Stage(body io.Reader, size int64, sha256Hex string) (*Staged, error) {
	tmp, err := os.CreateTemp(s.tmpDir(), "upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	fail := func(cause error) (*Staged, error) {
		return nil, errors.Join(cause, tmp.Close(), removeIfExists(tmpPath))
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(body, size+1))
	if err != nil {
		return fail(fmt.Errorf("read upload: %w", err))
	}
	if written != size {
		return fail(ErrSizeMismatch)
	}
	if !hashMatches(hasher.Sum(nil), sha256Hex) {
		return fail(ErrHashMismatch)
	}
	if err := tmp.Sync(); err != nil {
		return fail(fmt.Errorf("sync temporary file: %w", err))
	}
	if err := tmp.Chmod(0o444); err != nil {
		return fail(fmt.Errorf("set file mode: %w", err))
	}
	if err := tmp.Close(); err != nil {
		return nil, errors.Join(fmt.Errorf("close temporary file: %w", err), removeIfExists(tmpPath))
	}
	return &Staged{path: tmpPath}, nil
}

func (st *Staged) Discard() error {
	return removeIfExists(st.path)
}

func (st *Staged) Commit(s *Store, siteID, versionID, rel string) error {
	if err := sitepath.Validate(rel); err != nil {
		return errors.Join(err, st.Discard())
	}
	filesDir, err := s.filesDir(siteID, versionID)
	if err != nil {
		return errors.Join(err, st.Discard())
	}
	if err := place(filesDir, rel, st.path); err != nil {
		return errors.Join(err, st.Discard())
	}
	return nil
}

func hashMatches(sum []byte, wantHex string) bool {
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(sum, want) == 1
}

func place(filesDir, rel, tmpPath string) error {
	if err := os.MkdirAll(filesDir, dirMode); err != nil {
		return fmt.Errorf("create files directory: %w", err)
	}
	root, err := os.OpenRoot(filesDir)
	if err != nil {
		return fmt.Errorf("open files directory: %w", err)
	}
	if err := ensureDirectories(root, rel); err != nil {
		return errors.Join(err, root.Close())
	}
	if err := checkTarget(root, rel); err != nil {
		return errors.Join(err, root.Close())
	}
	if err := root.Close(); err != nil {
		return fmt.Errorf("close files directory: %w", err)
	}
	target := filepath.Join(filesDir, filepath.FromSlash(rel))
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("move file into place: %w", err)
	}
	return nil
}

func ensureDirectories(root *os.Root, rel string) error {
	segments := strings.Split(rel, "/")
	current := ""
	for _, segment := range segments[:len(segments)-1] {
		current = joinRel(current, segment)
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if err := root.Mkdir(current, dirMode); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: a parent of %q is not a directory", ErrNotRegular, rel)
		}
	}
	return nil
}

func checkTarget(root *os.Root, rel string) error {
	info, err := root.Lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect target: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %q", ErrNotRegular, rel)
	}
	return nil
}

func joinRel(base, segment string) string {
	if base == "" {
		return segment
	}
	return base + "/" + segment
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", filepath.Base(path), err)
	}
	return nil
}
