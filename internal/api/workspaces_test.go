package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestWorkspaceLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		sam := h.bearerActor("sam@example.com")

		list := decodeBody[workspaceListResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/workspaces", nil))
		if len(list.Workspaces) != 1 || list.Workspaces[0].Kind != "personal" || list.Workspaces[0].Role != "admin" || list.Workspaces[0].ScopedOnly {
			t.Fatalf("workspaces = %+v", list.Workspaces)
		}
		personal := list.Workspaces[0]

		acme := alex.createWorkspace("Acme Ltd", "")
		if acme.Slug != "acme-ltd" || acme.Kind != "shared" || acme.Role != "admin" {
			t.Errorf("created workspace = %+v", acme)
		}
		alex.expectError(http.StatusConflict, codeSlugTaken, "POST", "/api/v1/workspaces", createWorkspaceRequest{Name: "Again", Slug: "acme-ltd"})
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/workspaces", createWorkspaceRequest{Name: ""})
		alex.expectError(http.StatusBadRequest, codeInvalidJSON, "POST", "/api/v1/workspaces", `{"name":"x","extra":1}`)
		sam.createWorkspace("Sam team", "sam-team")

		byID := decodeBody[workspaceEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/workspaces/"+acme.ID, nil))
		if byID.Workspace.Slug != acme.Slug {
			t.Errorf("get by id = %+v", byID.Workspace)
		}
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/workspaces/acme-ltd", nil)
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/workspaces/"+personal.Slug, nil)

		renamed := decodeBody[workspaceEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/workspaces/acme-ltd", renameWorkspaceRequest{Name: "Acme Group"}))
		if renamed.Workspace.Name != "Acme Group" {
			t.Errorf("renamed = %+v", renamed.Workspace)
		}

		member := alex.addMember("acme-ltd", addMemberRequest{Email: "sam@example.com", Role: "write"})
		if member.Email != "sam@example.com" || member.Role != "write" || member.ScopeType != "workspace" || member.ScopeID != nil {
			t.Errorf("member = %+v", member)
		}
		sam.expectError(http.StatusForbidden, codeForbidden, "PATCH", "/api/v1/workspaces/acme-ltd", renameWorkspaceRequest{Name: "Mine"})
		sam.expectError(http.StatusForbidden, codeForbidden, "DELETE", "/api/v1/workspaces/acme-ltd", nil)

		site := sam.createSite("acme-ltd", createSiteRequest{Slug: "launch"})
		alex.expectError(http.StatusConflict, codeWorkspaceNotEmpty, "DELETE", "/api/v1/workspaces/acme-ltd", nil)
		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/sites/"+site.Slug, nil)
		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/workspaces/acme-ltd", nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/workspaces/acme-ltd", nil)

		alex.expectError(http.StatusConflict, codePersonalWorkspace, "DELETE", "/api/v1/workspaces/"+personal.Slug, nil)
		alex.expectError(http.StatusConflict, codePersonalWorkspace, "POST", "/api/v1/workspaces/"+personal.Slug+"/members",
			addMemberRequest{Email: "sam@example.com", Role: "read"})
	})
}

func TestMemberEndpoints(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		alex.createWorkspace("Acme Ltd", "acme")
		folder := alex.createAcmeFolder("Reports")

		alex.expectError(http.StatusNotFound, codeUserNotFound, "POST", "/api/v1/workspaces/acme/members", addMemberRequest{Email: "nobody@example.com", Role: "read"})
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/workspaces/acme/members", addMemberRequest{Email: "sam@example.com", Role: "owner"})
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/workspaces/acme/members",
			addMemberRequest{Email: "sam@example.com", Role: "admin", ScopeType: "folder", ScopeID: folder.ID})

		scoped := alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read", ScopeType: "folder", ScopeID: folder.ID})
		if scoped.ScopeID == nil || *scoped.ScopeID != folder.ID || scoped.ScopeName == nil || *scoped.ScopeName != "Reports" {
			t.Errorf("scoped member = %+v", scoped)
		}
		alex.expectError(http.StatusConflict, codeMemberExists, "POST", "/api/v1/workspaces/acme/members",
			addMemberRequest{Email: "sam@example.com", Role: "write", ScopeType: "folder", ScopeID: folder.ID})
		sam.expectError(http.StatusForbidden, codeForbidden, "GET", "/api/v1/workspaces/acme/members", nil)

		updated := decodeBody[memberEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/workspaces/acme/members/"+scoped.ID, updateMemberRequest{Role: "write"}))
		if updated.Member.Role != "write" {
			t.Errorf("updated = %+v", updated.Member)
		}

		members := decodeBody[memberListResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/workspaces/acme/members", nil))
		if len(members.Members) != 2 {
			t.Fatalf("members = %+v", members.Members)
		}
		var owner memberResponse
		for _, m := range members.Members {
			if m.Email == "alex@example.com" {
				owner = m
			}
		}
		alex.expectError(http.StatusConflict, codeLastAdmin, "DELETE", "/api/v1/workspaces/acme/members/"+owner.ID, nil)
		alex.expectError(http.StatusConflict, codeLastAdmin, "PATCH", "/api/v1/workspaces/acme/members/"+owner.ID, updateMemberRequest{Role: "read"})
		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/workspaces/acme/members/"+scoped.ID, nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/workspaces/acme/members/"+scoped.ID, nil)
		sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/workspaces/acme", nil)
	})
}

func TestFolderEndpoints(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")

		reports := alex.createAcmeFolder("Reports")
		alex.expectError(http.StatusConflict, codeFolderNameTaken, "POST", "/api/v1/workspaces/acme/folders", folderRequest{Name: "reports"})
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/workspaces/acme/folders", folderRequest{Name: " "})
		renamed := decodeBody[folderEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/workspaces/acme/folders/"+reports.ID, folderRequest{Name: "Quarterly"}))
		if renamed.Folder.Name != "Quarterly" {
			t.Errorf("renamed = %+v", renamed.Folder)
		}
		site := alex.createSite("acme", createSiteRequest{Slug: "q1", FolderID: reports.ID})
		if site.FolderID == nil || *site.FolderID != reports.ID {
			t.Fatalf("site folder = %v", site.FolderID)
		}
		list := decodeBody[folderListResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/workspaces/acme/folders", nil))
		if len(list.Folders) != 1 || list.Folders[0].Role != "admin" {
			t.Errorf("folders = %+v", list.Folders)
		}
		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/workspaces/acme/folders/"+reports.ID, nil)
		after := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/q1", nil))
		if after.Site.FolderID != nil {
			t.Errorf("site still filed in %v after the folder was deleted", *after.Site.FolderID)
		}
		alex.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/workspaces/acme/folders/"+reports.ID, nil)
	})
}

func TestWorkspaceCreationFollowsTheInstanceSetting(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		sam.createWorkspace("Open Co", "open-co")

		root.setSettings(`{"settings":{"workspaces.whoCanCreate":"root"}}`)
		sam.expectError(http.StatusForbidden, codeForbidden, "POST", "/api/v1/workspaces", createWorkspaceRequest{Name: "Closed Co", Slug: "closed-co"})
		root.createWorkspace("Acme Ltd", "acme")
		sam.expect(http.StatusOK, "GET", "/api/v1/workspaces/open-co", nil)

		root.setSettings(`{"settings":{"workspaces.whoCanCreate":"any"}}`)
		sam.createWorkspace("Closed Co", "closed-co")
	})
}
