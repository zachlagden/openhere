package sites

import (
	"time"

	"github.com/zachlagden/selfhost/internal/workspaces"
)

type Folder struct {
	ID          string
	WorkspaceID string
	Name        string
	Role        workspaces.Role
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Site struct {
	ID               string
	WorkspaceID      string
	WorkspaceSlug    string
	FolderID         string
	Slug             string
	DisplayName      string
	Description      string
	CreatedBy        string
	SpaMode          bool
	AccessMode       string
	CurrentVersionID string
	Role             workspaces.Role
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
