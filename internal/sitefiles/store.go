package sitefiles

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/zachlagden/openhere/internal/fsperm"
)

const (
	sitesDirName    = "sites"
	versionsDirName = "versions"
	filesDirName    = "files"
	tmpDirName      = "tmp"
	dirMode         = fsperm.DirMode
)

var (
	ErrNotFound     = errors.New("file not found")
	ErrNotRegular   = errors.New("not a regular file")
	ErrSizeMismatch = errors.New("file size does not match")
	ErrHashMismatch = errors.New("file hash does not match")
	ErrUnsafeID     = errors.New("identifier is not safe to use in a path")

	idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

type Store struct {
	root string
}

func New(dataDir string) (*Store, error) {
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	s := &Store{root: root}
	for _, dir := range []string{s.sitesDir(), s.tmpDir()} {
		if err := fsperm.EnsureDir(dir, nil); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) sitesDir() string { return filepath.Join(s.root, sitesDirName) }

func (s *Store) tmpDir() string { return filepath.Join(s.root, tmpDirName) }

func (s *Store) siteDir(siteID string) (string, error) {
	if !idPattern.MatchString(siteID) {
		return "", ErrUnsafeID
	}
	return filepath.Join(s.sitesDir(), siteID), nil
}

func (s *Store) versionDir(siteID, versionID string) (string, error) {
	site, err := s.siteDir(siteID)
	if err != nil {
		return "", err
	}
	if !idPattern.MatchString(versionID) {
		return "", ErrUnsafeID
	}
	return filepath.Join(site, versionsDirName, versionID), nil
}

func (s *Store) filesDir(siteID, versionID string) (string, error) {
	dir, err := s.versionDir(siteID, versionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filesDirName), nil
}
