package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/zachlagden/openhere/internal/fsperm"
)

const sqliteFileName = "selfhost.db"

func openSQLite(ctx context.Context, dataDir string) (Store, error) {
	if err := fsperm.EnsureDir(dataDir, nil); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	absolute, err := filepath.Abs(filepath.Join(dataDir, sqliteFileName))
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := fsperm.CreateFile(absolute, nil); err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	db, err := sql.Open("sqlite", sqliteDSN(absolute))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &sqlStore{db: db, driver: DriverSQLite, rebind: rebindForSQLite}
	if err := s.Ping(ctx); err != nil {
		if closeErr := s.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database: %w", closeErr))
		}
		return nil, err
	}
	restrictSQLiteFiles(absolute)
	return s, nil
}

func restrictSQLiteFiles(database string) {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		fsperm.Restrict(database+suffix, fsperm.FileMode, nil)
	}
}

func sqliteDSN(path string) string {
	query := url.Values{}
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	query.Set("_txlock", "immediate")
	dsn := url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}
	return dsn.String()
}
