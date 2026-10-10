package api

import (
	"encoding/json"
	"time"

	"github.com/zachlagden/selfhost/internal/hosting"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type workspaceResponse struct {
	ID         string          `json:"id"`
	Slug       string          `json:"slug"`
	Name       string          `json:"name"`
	Kind       workspaces.Kind `json:"kind"`
	Role       workspaces.Role `json:"role"`
	ScopedOnly bool            `json:"scopedOnly"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type workspaceEnvelope struct {
	Workspace workspaceResponse `json:"workspace"`
}

type workspaceListResponse struct {
	Workspaces []workspaceResponse `json:"workspaces"`
}

func toWorkspaceResponse(a workspaces.Access) workspaceResponse {
	ws := a.Workspace
	return workspaceResponse{
		ID: ws.ID, Slug: ws.Slug, Name: ws.Name, Kind: ws.Kind, Role: a.WorkspaceRole(), ScopedOnly: a.ScopedOnly(),
		CreatedAt: ws.CreatedAt, UpdatedAt: ws.UpdatedAt,
	}
}

type memberResponse struct {
	ID        string               `json:"id"`
	UserID    string               `json:"userId"`
	Email     string               `json:"email"`
	Role      workspaces.Role      `json:"role"`
	ScopeType workspaces.ScopeType `json:"scopeType"`
	ScopeID   *string              `json:"scopeId"`
	ScopeName *string              `json:"scopeName"`
	CreatedAt time.Time            `json:"createdAt"`
}

type memberEnvelope struct {
	Member memberResponse `json:"member"`
}

type memberListResponse struct {
	Members []memberResponse `json:"members"`
}

func toMemberResponse(m workspaces.Member) memberResponse {
	response := memberResponse{
		ID: m.ID, UserID: m.UserID, Email: m.Email, Role: m.Role, ScopeType: m.ScopeType, CreatedAt: m.CreatedAt,
	}
	switch m.ScopeType {
	case workspaces.ScopeFolder:
		response.ScopeID, response.ScopeName = &m.FolderID, &m.ScopeName
	case workspaces.ScopeSite:
		response.ScopeID, response.ScopeName = &m.SiteID, &m.ScopeName
	}
	return response
}

type folderResponse struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspaceId"`
	Name        string          `json:"name"`
	Role        workspaces.Role `json:"role"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type folderEnvelope struct {
	Folder folderResponse `json:"folder"`
}

type folderListResponse struct {
	Folders []folderResponse `json:"folders"`
}

func toFolderResponse(f sites.Folder) folderResponse {
	return folderResponse{ID: f.ID, WorkspaceID: f.WorkspaceID, Name: f.Name, Role: f.Role, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt}
}

type siteResponse struct {
	ID               string              `json:"id"`
	WorkspaceID      string              `json:"workspaceId"`
	WorkspaceSlug    string              `json:"workspaceSlug"`
	FolderID         *string             `json:"folderId"`
	Slug             string              `json:"slug"`
	DisplayName      string              `json:"displayName"`
	Description      string              `json:"description"`
	CreatedBy        *string             `json:"createdBy"`
	SpaMode          bool                `json:"spaMode"`
	AccessMode       string              `json:"accessMode"`
	CurrentVersionID *string             `json:"currentVersionId"`
	URL              *string             `json:"url"`
	Hosting          siteHostingResponse `json:"hosting"`
	Role             workspaces.Role     `json:"role"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

type siteEnvelope struct {
	Site siteResponse `json:"site"`
}

type siteListResponse struct {
	Sites      []siteResponse `json:"sites"`
	NextCursor *string        `json:"nextCursor"`
}

func toSiteResponse(s sites.Site, availability hosting.SiteAvailability) siteResponse {
	return siteResponse{
		ID: s.ID, WorkspaceID: s.WorkspaceID, WorkspaceSlug: s.WorkspaceSlug, FolderID: optional(s.FolderID),
		Slug: s.Slug, DisplayName: s.DisplayName, Description: s.Description, CreatedBy: optional(s.CreatedBy),
		SpaMode: s.SpaMode, AccessMode: s.AccessMode, CurrentVersionID: optional(s.CurrentVersionID), URL: optional(availability.URL),
		Hosting: toSiteHostingResponse(availability),
		Role:    s.Role, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

type optionalString struct {
	Set   bool
	Value *string
}

func (o *optionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}
