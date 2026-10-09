package sitefiles

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/zachlagden/selfhost/internal/sitepath"
)

type Expected struct {
	Path   string
	Size   int64
	SHA256 string
}

func (s *Store) Open(siteID, versionID, rel string) (*os.File, os.FileInfo, error) {
	if err := sitepath.Validate(rel); err != nil {
		return nil, nil, err
	}
	filesDir, err := s.filesDir(siteID, versionID)
	if err != nil {
		return nil, nil, err
	}
	root, err := os.OpenRoot(filesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("open files directory: %w", err)
	}
	file, info, err := openRegular(root, rel)
	if err != nil {
		return nil, nil, errors.Join(err, root.Close())
	}
	if err := root.Close(); err != nil {
		return nil, nil, errors.Join(fmt.Errorf("close files directory: %w", err), file.Close())
	}
	return file, info, nil
}

func checkParents(root *os.Root, rel string) error {
	segments := strings.Split(rel, "/")
	current := ""
	for _, segment := range segments[:len(segments)-1] {
		current = joinRel(current, segment)
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
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

func openRegular(root *os.Root, rel string) (*os.File, os.FileInfo, error) {
	if err := checkParents(root, rel); err != nil {
		return nil, nil, err
	}
	linkInfo, err := root.Lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("inspect file: %w", err)
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, nil, ErrNotRegular
	}
	file, err := root.Open(rel)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("stat file: %w", err), file.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errors.Join(ErrNotRegular, file.Close())
	}
	return file, info, nil
}

func (s *Store) Verify(siteID, versionID string, want Expected) error {
	file, info, err := s.Open(siteID, versionID, want.Path)
	if err != nil {
		return err
	}
	if info.Size() != want.Size {
		return errors.Join(ErrSizeMismatch, file.Close())
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return errors.Join(fmt.Errorf("read file: %w", err), file.Close())
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	if !hashMatches(hasher.Sum(nil), want.SHA256) {
		return ErrHashMismatch
	}
	return nil
}
