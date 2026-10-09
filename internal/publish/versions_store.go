package publish

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	versionColumns = "id, site_id, status, created_by, created_at, finalized_at, file_count, total_bytes, base_version_id"
	fileColumns    = "path, size, sha256, content_type"
	insertChunk    = 400
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVersion(row rowScanner) (Version, error) {
	var v Version
	var createdBy, base sql.NullString
	var finalizedAt sql.NullTime
	err := row.Scan(&v.ID, &v.SiteID, &v.Status, &createdBy, &v.CreatedAt, &finalizedAt, &v.FileCount, &v.TotalBytes, &base)
	if err != nil {
		return Version{}, err
	}
	v.CreatedBy = createdBy.String
	v.BaseVersionID = base.String
	if finalizedAt.Valid {
		at := finalizedAt.Time
		v.FinalizedAt = &at
	}
	return v, nil
}

func loadVersion(ctx context.Context, q store.Querier, siteID, versionID string) (Version, error) {
	v, err := scanVersion(q.QueryRowContext(ctx,
		"SELECT "+versionColumns+" FROM versions WHERE id = $1 AND site_id = $2", versionID, siteID))
	if errors.Is(err, sql.ErrNoRows) {
		return Version{}, workspaces.ErrNotFound
	}
	if err != nil {
		return Version{}, fmt.Errorf("load version: %w", err)
	}
	return v, nil
}

func insertVersion(ctx context.Context, q store.Querier, v Version, files []FileSpec) error {
	var createdBy, base sql.NullString
	if v.CreatedBy != "" {
		createdBy = sql.NullString{String: v.CreatedBy, Valid: true}
	}
	if v.BaseVersionID != "" {
		base = sql.NullString{String: v.BaseVersionID, Valid: true}
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO versions (id, site_id, status, created_by, created_at, finalized_at, file_count, total_bytes, base_version_id)
		VALUES ($1, $2, $3, $4, $5, NULL, $6, $7, $8)`,
		v.ID, v.SiteID, StatusPending, createdBy, v.CreatedAt, v.FileCount, v.TotalBytes, base)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}
	for start := 0; start < len(files); start += insertChunk {
		end := min(start+insertChunk, len(files))
		if err := insertFiles(ctx, q, v.ID, files[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func insertFiles(ctx context.Context, q store.Querier, versionID string, files []FileSpec) error {
	var query strings.Builder
	query.WriteString("INSERT INTO version_files (version_id, path, size, sha256, content_type) VALUES ")
	args := make([]any, 0, len(files)*5)
	for i, file := range files {
		if i > 0 {
			query.WriteString(", ")
		}
		base := i * 5
		query.WriteString("($" + strconv.Itoa(base+1) + ", $" + strconv.Itoa(base+2) + ", $" + strconv.Itoa(base+3) +
			", $" + strconv.Itoa(base+4) + ", $" + strconv.Itoa(base+5) + ")")
		args = append(args, versionID, file.Path, file.Size, file.SHA256, file.ContentType)
	}
	if _, err := q.ExecContext(ctx, query.String(), args...); err != nil {
		return fmt.Errorf("insert version files: %w", err)
	}
	return nil
}

func loadFiles(ctx context.Context, q store.Querier, versionID string) ([]FileSpec, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+fileColumns+" FROM version_files WHERE version_id = $1 ORDER BY path", versionID)
	if err != nil {
		return nil, fmt.Errorf("load version files: %w", err)
	}
	files := []FileSpec{}
	for rows.Next() {
		var f FileSpec
		if err := rows.Scan(&f.Path, &f.Size, &f.SHA256, &f.ContentType); err != nil {
			return nil, errors.Join(fmt.Errorf("scan version file: %w", err), rows.Close())
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read version files: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close version file rows: %w", err)
	}
	return files, nil
}

func loadFile(ctx context.Context, q store.Querier, versionID, filePath string) (FileSpec, error) {
	var f FileSpec
	err := q.QueryRowContext(ctx,
		"SELECT "+fileColumns+" FROM version_files WHERE version_id = $1 AND path = $2", versionID, filePath).
		Scan(&f.Path, &f.Size, &f.SHA256, &f.ContentType)
	if errors.Is(err, sql.ErrNoRows) {
		return FileSpec{}, ErrFileNotInList
	}
	if err != nil {
		return FileSpec{}, fmt.Errorf("load version file: %w", err)
	}
	return f, nil
}

func listVersions(ctx context.Context, q store.Querier, siteID string, limit int) ([]Version, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+versionColumns+" FROM versions WHERE site_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2", siteID, limit)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	versions := []Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("scan version: %w", err), rows.Close())
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read versions: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close version rows: %w", err)
	}
	return versions, nil
}

func deleteVersionRow(ctx context.Context, q store.Querier, siteID, versionID string) (bool, error) {
	result, err := q.ExecContext(ctx,
		`DELETE FROM versions WHERE id = $1 AND site_id = $2
		AND NOT EXISTS (SELECT 1 FROM sites WHERE current_version_id = $1)`, versionID, siteID)
	if err != nil {
		return false, fmt.Errorf("delete version: %w", err)
	}
	return rowsChanged(result, "delete version")
}

func rowsChanged(result sql.Result, action string) (bool, error) {
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("%s: %w", action, err)
	}
	return n > 0, nil
}

func expired(v Version, now time.Time) bool {
	return v.Status == StatusPending && now.After(v.ExpiresAt())
}
