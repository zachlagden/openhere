package hosting

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zachlagden/selfhost/internal/sitepath"
	"github.com/zachlagden/selfhost/internal/store"
)

type fileRecord struct {
	Path        string
	Size        int64
	SHA256      string
	ContentType string
}

type fileIndex struct {
	db store.Querier
}

func (x fileIndex) find(ctx context.Context, versionID string, paths []string) (map[string]fileRecord, error) {
	args := make([]any, 0, len(paths)+1)
	args = append(args, versionID)
	placeholders := make([]string, len(paths))
	for i, p := range paths {
		args = append(args, p)
		placeholders[i] = "$" + strconv.Itoa(i+2)
	}
	rows, err := x.db.QueryContext(ctx,
		"SELECT path, size, sha256, content_type FROM version_files WHERE version_id = $1 AND path IN ("+strings.Join(placeholders, ", ")+")",
		args...)
	if err != nil {
		return nil, fmt.Errorf("look up files: %w", err)
	}
	found := make(map[string]fileRecord, len(paths))
	for rows.Next() {
		var rec fileRecord
		if err := rows.Scan(&rec.Path, &rec.Size, &rec.SHA256, &rec.ContentType); err != nil {
			return nil, closeAfter(rows, fmt.Errorf("scan file: %w", err))
		}
		found[rec.Path] = rec
	}
	if err := rows.Err(); err != nil {
		return nil, closeAfter(rows, fmt.Errorf("read files: %w", err))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close file rows: %w", err)
	}
	return found, nil
}

func closeAfter(rows interface{ Close() error }, cause error) error {
	if err := rows.Close(); err != nil {
		return errors.Join(cause, fmt.Errorf("close file rows: %w", err))
	}
	return cause
}

type lookupPlan struct {
	candidates []string
	dirIndex   string
}

func planLookup(requestPath string) (lookupPlan, bool) {
	if requestPath == "/" {
		return lookupPlan{candidates: []string{"index.html"}}, true
	}
	if len(requestPath) < 2 || requestPath[0] != '/' {
		return lookupPlan{}, false
	}
	rel := requestPath[1:]
	var plan lookupPlan
	if strings.HasSuffix(rel, "/") {
		rel = strings.TrimSuffix(rel, "/")
		plan.candidates = []string{rel + "/index.html", rel + ".html"}
	} else {
		plan.dirIndex = rel + "/index.html"
		plan.candidates = []string{rel, rel + ".html", plan.dirIndex}
	}
	for _, candidate := range plan.candidates {
		if sitepath.Validate(candidate) != nil {
			return lookupPlan{}, false
		}
	}
	return plan, true
}

type resolution struct {
	record   fileRecord
	redirect bool
}

func (x fileIndex) resolve(ctx context.Context, versionID, requestPath string) (resolution, bool, error) {
	plan, ok := planLookup(requestPath)
	if !ok {
		return resolution{}, false, nil
	}
	found, err := x.find(ctx, versionID, plan.candidates)
	if err != nil {
		return resolution{}, false, err
	}
	for _, candidate := range plan.candidates {
		if rec, hit := found[candidate]; hit {
			return resolution{record: rec, redirect: candidate == plan.dirIndex}, true, nil
		}
	}
	return resolution{}, false, nil
}

func (x fileIndex) one(ctx context.Context, versionID, rel string) (fileRecord, bool, error) {
	found, err := x.find(ctx, versionID, []string{rel})
	if err != nil {
		return fileRecord{}, false, err
	}
	rec, ok := found[rel]
	return rec, ok, nil
}
