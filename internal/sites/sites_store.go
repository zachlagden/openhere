package sites

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const siteColumns = "s.id, s.workspace_id, w.slug, s.folder_id, s.slug, s.display_name, s.description, s.created_by, s.created_at, s.updated_at, s.current_version_id, s.spa_mode, s.access_mode"

const siteFrom = " FROM sites s JOIN workspaces w ON w.id = s.workspace_id"

func scanSite(row interface{ Scan(dest ...any) error }, extra ...any) (Site, error) {
	var site Site
	var folderID, createdBy, currentVersion sql.NullString
	dest := append([]any{
		&site.ID, &site.WorkspaceID, &site.WorkspaceSlug, &folderID, &site.Slug, &site.DisplayName,
		&site.Description, &createdBy, &site.CreatedAt, &site.UpdatedAt, &currentVersion, &site.SpaMode, &site.AccessMode,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return Site{}, err
	}
	site.FolderID = folderID.String
	site.CreatedBy = createdBy.String
	site.CurrentVersionID = currentVersion.String
	return site, nil
}

func loadSite(ctx context.Context, q store.Querier, ref string) (Site, error) {
	site, err := scanSite(q.QueryRowContext(ctx, "SELECT "+siteColumns+siteFrom+" WHERE s.id = $1 OR s.slug = $1", ref))
	if errors.Is(err, sql.ErrNoRows) {
		return Site{}, workspaces.ErrNotFound
	}
	if err != nil {
		return Site{}, fmt.Errorf("load site: %w", err)
	}
	return site, nil
}

func insertSite(ctx context.Context, q store.Querier, site Site) error {
	var folderID, createdBy sql.NullString
	if site.FolderID != "" {
		folderID = sql.NullString{String: site.FolderID, Valid: true}
	}
	if site.CreatedBy != "" {
		createdBy = sql.NullString{String: site.CreatedBy, Valid: true}
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO sites (id, workspace_id, folder_id, slug, display_name, description, created_by, created_at, updated_at, spa_mode, access_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9, $10)`,
		site.ID, site.WorkspaceID, folderID, site.Slug, site.DisplayName, site.Description, createdBy, site.CreatedAt, site.SpaMode, site.AccessMode)
	if err != nil {
		return fmt.Errorf("insert site: %w", err)
	}
	return nil
}
