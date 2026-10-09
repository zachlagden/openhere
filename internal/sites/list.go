package sites

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	FolderNone   = "none"
	DefaultLimit = 50
	MaxLimit     = 200
)

type Filter struct {
	Workspace string
	Folder    string
	Search    string
	Limit     int
	Cursor    string
}

type Page struct {
	Sites      []Site
	NextCursor string
}

type queryBuilder struct {
	args       []any
	conditions []string
}

func (b *queryBuilder) arg(value any) string {
	b.args = append(b.args, value)
	return "$" + strconv.Itoa(len(b.args))
}

func (b *queryBuilder) where(condition string) {
	b.conditions = append(b.conditions, condition)
}

func escapeLike(term string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(term)
}

func (s *Service) ListSites(ctx context.Context, userID string, f Filter) (Page, error) {
	limit, err := normalizeLimit(f.Limit)
	if err != nil {
		return Page{}, err
	}
	b := &queryBuilder{}
	user := b.arg(userID)
	b.where(workspaces.SiteCoverSQL("s", user))
	if err := s.addFilters(ctx, b, userID, f); err != nil {
		return Page{}, err
	}
	query := "SELECT " + siteColumns + ", " + workspaces.SiteRankSQL("s", user) + siteFrom +
		" WHERE " + strings.Join(b.conditions, " AND ") +
		" ORDER BY s.created_at DESC, s.id DESC LIMIT " + b.arg(limit+1)
	return s.runList(ctx, query, b.args, limit)
}

func normalizeLimit(limit int) (int, error) {
	switch {
	case limit == 0:
		return DefaultLimit, nil
	case limit < 0 || limit > MaxLimit:
		return 0, validate.Invalid(fmt.Sprintf("Limit must be between 1 and %d.", MaxLimit))
	default:
		return limit, nil
	}
}

func (s *Service) addFilters(ctx context.Context, b *queryBuilder, userID string, f Filter) error {
	if f.Workspace != "" {
		acc, err := s.ws.Access(ctx, userID, f.Workspace)
		if err != nil {
			return err
		}
		b.where("s.workspace_id = " + b.arg(acc.Workspace.ID))
	}
	switch f.Folder {
	case "":
	case FolderNone:
		b.where("s.folder_id IS NULL")
	default:
		b.where("s.folder_id = " + b.arg(f.Folder))
	}
	if term := strings.TrimSpace(f.Search); term != "" {
		pattern := b.arg("%" + escapeLike(strings.ToLower(term)) + "%")
		b.where("(lower(s.display_name) LIKE " + pattern + ` ESCAPE '\' OR lower(s.slug) LIKE ` + pattern + ` ESCAPE '\')`)
	}
	if f.Cursor != "" {
		after, err := decodeCursor(f.Cursor)
		if err != nil {
			return err
		}
		stamp, id := b.arg(after.createdAt), b.arg(after.id)
		b.where("(s.created_at < " + stamp + " OR (s.created_at = " + stamp + " AND s.id < " + id + "))")
	}
	return nil
}

func (s *Service) runList(ctx context.Context, query string, args []any, limit int) (Page, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("list sites: %w", err)
	}
	page := Page{Sites: []Site{}}
	for rows.Next() {
		var rank sql.NullInt64
		site, err := scanSite(rows, &rank)
		if err != nil {
			return Page{}, errors.Join(fmt.Errorf("scan site: %w", err), rows.Close())
		}
		site.Role = workspaces.RoleFromRank(int(rank.Int64))
		page.Sites = append(page.Sites, site)
	}
	if err := rows.Err(); err != nil {
		return Page{}, errors.Join(fmt.Errorf("read sites: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return Page{}, fmt.Errorf("close site rows: %w", err)
	}
	if len(page.Sites) > limit {
		page.Sites = page.Sites[:limit]
		page.NextCursor = encodeCursor(page.Sites[limit-1])
	}
	return page, nil
}
