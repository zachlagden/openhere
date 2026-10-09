package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const sqliteFileName = "selfhost.db"

func openSQLite(ctx context.Context, dataDir string) (Store, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	absolute, err := filepath.Abs(filepath.Join(dataDir, sqliteFileName))
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
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
	return s, nil
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
