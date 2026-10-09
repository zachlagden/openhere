package workspaces

type ResourceKind string

const (
	ResourceWorkspace ResourceKind = "workspace"
	ResourceFolder    ResourceKind = "folder"
	ResourceSite      ResourceKind = "site"
)

type Resource struct {
	Kind        ResourceKind
	WorkspaceID string
	FolderID    string
	SiteID      string
}

func WorkspaceResource(workspaceID string) Resource {
	return Resource{Kind: ResourceWorkspace, WorkspaceID: workspaceID}
}

func FolderResource(workspaceID, folderID string) Resource {
	return Resource{Kind: ResourceFolder, WorkspaceID: workspaceID, FolderID: folderID}
}

func SiteResource(workspaceID, siteID, folderID string) Resource {
	return Resource{Kind: ResourceSite, WorkspaceID: workspaceID, SiteID: siteID, FolderID: folderID}
}

func EffectiveRole(grants []Grant, res Resource) Role {
	best := RoleNone
	inWorkspace := false
	for _, g := range grants {
		if g.WorkspaceID != res.WorkspaceID {
			continue
		}
		inWorkspace = true
		if covers(g, res) && g.Role.rank() > best.rank() {
			best = g.Role
		}
	}
	if best == RoleNone && inWorkspace && res.Kind == ResourceWorkspace {
		return RoleRead
	}
	return best
}

func covers(g Grant, res Resource) bool {
	switch g.ScopeType {
	case ScopeWorkspace:
		return true
	case ScopeFolder:
		return res.FolderID != "" && g.FolderID == res.FolderID && res.Kind != ResourceWorkspace
	case ScopeSite:
		return res.Kind == ResourceSite && res.SiteID != "" && g.SiteID == res.SiteID
	default:
		return false
	}
}

func Require(have, need Role) error {
	if have == RoleNone {
		return ErrNotFound
	}
	if !have.AtLeast(need) {
		return ErrForbidden
	}
	return nil
}

func SiteCoverSQL(siteAlias, userPlaceholder string) string {
	return "EXISTS (SELECT 1 FROM workspace_access a WHERE a.user_id = " + userPlaceholder +
		" AND a.workspace_id = " + siteAlias + ".workspace_id AND (a.scope_type = 'workspace' OR a.site_id = " + siteAlias +
		".id OR a.folder_id = " + siteAlias + ".folder_id))"
}

func SiteRankSQL(siteAlias, userPlaceholder string) string {
	return "(SELECT MAX(CASE a.role WHEN 'admin' THEN 3 WHEN 'write' THEN 2 ELSE 1 END) FROM workspace_access a WHERE a.user_id = " +
		userPlaceholder + " AND a.workspace_id = " + siteAlias + ".workspace_id AND (a.scope_type = 'workspace' OR a.site_id = " +
		siteAlias + ".id OR a.folder_id = " + siteAlias + ".folder_id))"
}
