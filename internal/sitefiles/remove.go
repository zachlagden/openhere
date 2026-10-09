package sitefiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func (s *Store) RemoveVersion(siteID, versionID string) error {
	dir, err := s.versionDir(siteID, versionID)
	if err != nil {
		return err
	}
	return removeTree(dir)
}

func (s *Store) RemoveSite(siteID string) error {
	dir, err := s.siteDir(siteID)
	if err != nil {
		return err
	}
	return removeTree(dir)
}

func removeTree(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", filepath.Base(dir), err)
	}
	return nil
}

func (s *Store) SiteDirs() ([]string, error) {
	return listIDs(s.sitesDir())
}

func (s *Store) VersionDirs(siteID string) ([]string, error) {
	site, err := s.siteDir(siteID)
	if err != nil {
		return nil, err
	}
	return listIDs(filepath.Join(site, versionsDirName))
}

func listIDs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", filepath.Base(dir), err)
	}
	var ids []string
	for _, entry := range entries {
		if idPattern.MatchString(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}
	return ids, nil
}

func (s *Store) SweepTemporary(olderThan time.Time) (int, error) {
	entries, err := os.ReadDir(s.tmpDir())
	if err != nil {
		return 0, fmt.Errorf("list temporary files: %w", err)
	}
	removed := 0
	var problems []error
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				problems = append(problems, err)
			}
			continue
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(olderThan) {
			continue
		}
		if err := removeIfExists(filepath.Join(s.tmpDir(), entry.Name())); err != nil {
			problems = append(problems, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(problems...)
}
