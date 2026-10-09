package workspaces

import "time"

type Kind string

const (
	KindPersonal Kind = "personal"
	KindShared   Kind = "shared"
)

type Role string

const (
	RoleNone  Role = "none"
	RoleRead  Role = "read"
	RoleWrite Role = "write"
	RoleAdmin Role = "admin"
)

func (r Role) rank() int {
	switch r {
	case RoleRead:
		return 1
	case RoleWrite:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}

func (r Role) AtLeast(min Role) bool { return r.rank() >= min.rank() }

func ParseRole(raw string) (Role, bool) {
	switch role := Role(raw); role {
	case RoleRead, RoleWrite, RoleAdmin:
		return role, true
	default:
		return RoleNone, false
	}
}

func RoleFromRank(rank int) Role {
	switch rank {
	case 1:
		return RoleRead
	case 2:
		return RoleWrite
	case 3:
		return RoleAdmin
	default:
		return RoleNone
	}
}

type ScopeType string

const (
	ScopeWorkspace ScopeType = "workspace"
	ScopeFolder    ScopeType = "folder"
	ScopeSite      ScopeType = "site"
)

type Workspace struct {
	ID        string
	Slug      string
	Name      string
	Kind      Kind
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Grant struct {
	ID          string
	WorkspaceID string
	UserID      string
	Role        Role
	ScopeType   ScopeType
	FolderID    string
	SiteID      string
}

type Member struct {
	Grant
	Email     string
	ScopeName string
	CreatedAt time.Time
}
