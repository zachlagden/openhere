package workspaces

import "database/sql"

const (
	workspaceColumns = "w.id, w.slug, w.name, w.kind, w.created_by, w.created_at, w.updated_at"
	grantColumns     = "id, workspace_id, user_id, role, scope_type, folder_id, site_id"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanWorkspace(row scanner) (Workspace, error) {
	var ws Workspace
	var createdBy sql.NullString
	if err := row.Scan(&ws.ID, &ws.Slug, &ws.Name, &ws.Kind, &createdBy, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
		return Workspace{}, err
	}
	ws.CreatedBy = createdBy.String
	return ws, nil
}

func scanGrant(row scanner) (Grant, error) {
	var g Grant
	var folderID, siteID sql.NullString
	if err := row.Scan(&g.ID, &g.WorkspaceID, &g.UserID, &g.Role, &g.ScopeType, &folderID, &siteID); err != nil {
		return Grant{}, err
	}
	g.FolderID = folderID.String
	g.SiteID = siteID.String
	return g, nil
}
