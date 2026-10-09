package workspaces

import (
	"errors"
	"testing"
)

const matrixWorkspace = "ws1"

type matrixResources struct {
	workspace, folderA, folderB, siteInA, siteInB, siteUnfiled, siteTarget Role
}

func workspaceWide(role Role) Grant {
	return Grant{WorkspaceID: matrixWorkspace, Role: role, ScopeType: ScopeWorkspace}
}

func folderGrant(role Role, folderID string) Grant {
	return Grant{WorkspaceID: matrixWorkspace, Role: role, ScopeType: ScopeFolder, FolderID: folderID}
}

func siteGrant(role Role, siteID string) Grant {
	return Grant{WorkspaceID: matrixWorkspace, Role: role, ScopeType: ScopeSite, SiteID: siteID}
}

func resolveMatrix(grants []Grant) matrixResources {
	return matrixResources{
		workspace:   EffectiveRole(grants, WorkspaceResource(matrixWorkspace)),
		folderA:     EffectiveRole(grants, FolderResource(matrixWorkspace, "folderA")),
		folderB:     EffectiveRole(grants, FolderResource(matrixWorkspace, "folderB")),
		siteInA:     EffectiveRole(grants, SiteResource(matrixWorkspace, "siteInA", "folderA")),
		siteInB:     EffectiveRole(grants, SiteResource(matrixWorkspace, "siteInB", "folderB")),
		siteUnfiled: EffectiveRole(grants, SiteResource(matrixWorkspace, "siteUnfiled", "")),
		siteTarget:  EffectiveRole(grants, SiteResource(matrixWorkspace, "siteTarget", "")),
	}
}

func TestEffectiveRoleMatrix(t *testing.T) {
	const none = RoleNone
	tests := []struct {
		name   string
		grants []Grant
		want   matrixResources
	}{
		{"no grants", nil, matrixResources{none, none, none, none, none, none, none}},
		{"workspace admin", []Grant{workspaceWide(RoleAdmin)},
			matrixResources{RoleAdmin, RoleAdmin, RoleAdmin, RoleAdmin, RoleAdmin, RoleAdmin, RoleAdmin}},
		{"workspace write", []Grant{workspaceWide(RoleWrite)},
			matrixResources{RoleWrite, RoleWrite, RoleWrite, RoleWrite, RoleWrite, RoleWrite, RoleWrite}},
		{"workspace read", []Grant{workspaceWide(RoleRead)},
			matrixResources{RoleRead, RoleRead, RoleRead, RoleRead, RoleRead, RoleRead, RoleRead}},
		{"folder write", []Grant{folderGrant(RoleWrite, "folderA")},
			matrixResources{RoleRead, RoleWrite, none, RoleWrite, none, none, none}},
		{"folder read", []Grant{folderGrant(RoleRead, "folderA")},
			matrixResources{RoleRead, RoleRead, none, RoleRead, none, none, none}},
		{"site write", []Grant{siteGrant(RoleWrite, "siteTarget")},
			matrixResources{RoleRead, none, none, none, none, none, RoleWrite}},
		{"site read", []Grant{siteGrant(RoleRead, "siteInB")},
			matrixResources{RoleRead, none, none, none, RoleRead, none, none}},
		{"workspace read plus folder write", []Grant{workspaceWide(RoleRead), folderGrant(RoleWrite, "folderA")},
			matrixResources{RoleRead, RoleWrite, RoleRead, RoleWrite, RoleRead, RoleRead, RoleRead}},
		{"workspace read plus site write", []Grant{workspaceWide(RoleRead), siteGrant(RoleWrite, "siteTarget")},
			matrixResources{RoleRead, RoleRead, RoleRead, RoleRead, RoleRead, RoleRead, RoleWrite}},
		{"workspace write plus folder read keeps write", []Grant{workspaceWide(RoleWrite), folderGrant(RoleRead, "folderA")},
			matrixResources{RoleWrite, RoleWrite, RoleWrite, RoleWrite, RoleWrite, RoleWrite, RoleWrite}},
		{"folder read plus site write inside it", []Grant{folderGrant(RoleRead, "folderA"), siteGrant(RoleWrite, "siteInA")},
			matrixResources{RoleRead, RoleRead, none, RoleWrite, none, none, none}},
		{"two folders with different roles", []Grant{folderGrant(RoleRead, "folderA"), folderGrant(RoleWrite, "folderB")},
			matrixResources{RoleRead, RoleRead, RoleWrite, RoleRead, RoleWrite, none, none}},
		{"folder write plus site read elsewhere", []Grant{folderGrant(RoleWrite, "folderA"), siteGrant(RoleRead, "siteUnfiled")},
			matrixResources{RoleRead, RoleWrite, none, RoleWrite, none, RoleRead, none}},
		{"grants in another workspace are ignored", []Grant{{WorkspaceID: "other", Role: RoleAdmin, ScopeType: ScopeWorkspace}},
			matrixResources{none, none, none, none, none, none, none}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveMatrix(tt.grants); got != tt.want {
				t.Errorf("roles = %+v\nwant    %+v", got, tt.want)
			}
		})
	}
}

func TestSiteGrantNeverCoversFolders(t *testing.T) {
	grants := []Grant{siteGrant(RoleWrite, "siteInA")}
	if got := EffectiveRole(grants, FolderResource(matrixWorkspace, "folderA")); got != RoleNone {
		t.Errorf("folder role from a site grant = %q, want none", got)
	}
}

func TestRequire(t *testing.T) {
	tests := []struct {
		have, need Role
		want       error
	}{
		{RoleNone, RoleRead, ErrNotFound},
		{RoleNone, RoleAdmin, ErrNotFound},
		{RoleRead, RoleWrite, ErrForbidden},
		{RoleWrite, RoleAdmin, ErrForbidden},
		{RoleRead, RoleRead, nil},
		{RoleAdmin, RoleWrite, nil},
	}
	for _, tt := range tests {
		if got := Require(tt.have, tt.need); !errors.Is(got, tt.want) {
			t.Errorf("Require(%s, %s) = %v, want %v", tt.have, tt.need, got, tt.want)
		}
	}
}

func TestParseRole(t *testing.T) {
	for _, valid := range []string{"admin", "write", "read"} {
		if role, ok := ParseRole(valid); !ok || string(role) != valid {
			t.Errorf("ParseRole(%q) = %q, %v", valid, role, ok)
		}
	}
	for _, invalid := range []string{"", "none", "owner", "ADMIN"} {
		if _, ok := ParseRole(invalid); ok {
			t.Errorf("ParseRole(%q) accepted", invalid)
		}
	}
}
