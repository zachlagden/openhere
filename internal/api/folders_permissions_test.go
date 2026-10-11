package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestFolderScopedWriterCannotDeleteTheFolderAndEraseOthersAccess(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")
		sam := h.bearerActor("sam@example.com")
		kim := h.bearerActor("kim@example.com")
		alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "write", ScopeType: "folder", ScopeID: reports.ID})
		alex.addMember("acme", addMemberRequest{Email: "kim@example.com", Role: "read", ScopeType: "folder", ScopeID: reports.ID})
		path := "/api/v1/workspaces/acme/folders/" + reports.ID

		sam.expectError(http.StatusForbidden, codeForbidden, "DELETE", path+"?force=true", nil)
		sam.expectError(http.StatusForbidden, codeForbidden, "PATCH", path, folderRequest{Name: "Mine"})

		kimFolders := decodeBody[folderListResponse](t, kim.expect(http.StatusOK, "GET", "/api/v1/workspaces/acme/folders", nil))
		if len(kimFolders.Folders) != 1 || kimFolders.Folders[0].Name != "Reports" {
			t.Errorf("another user's folder access changed: %+v", kimFolders.Folders)
		}
	})
}

func TestDeletingAFolderWithGrantsReturnsFolderHasGrantsUntilForced(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")
		h.bearerActor("sam@example.com")
		alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read", ScopeType: "folder", ScopeID: reports.ID})
		path := "/api/v1/workspaces/acme/folders/" + reports.ID

		alex.expectError(http.StatusConflict, codeFolderHasGrants, "DELETE", path, nil)
		alex.expectError(http.StatusConflict, codeFolderHasGrants, "DELETE", path+"?force=false", nil)
		alex.expect(http.StatusNoContent, "DELETE", path+"?force=true", nil)
	})
}

func TestWorkspaceWriterRenamesFoldersButCannotDeleteThem(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")
		kim := h.bearerActor("kim@example.com")
		alex.addMember("acme", addMemberRequest{Email: "kim@example.com", Role: "write"})
		path := "/api/v1/workspaces/acme/folders/" + reports.ID

		kim.expect(http.StatusOK, "PATCH", path, folderRequest{Name: "Quarterly"})
		kim.expectError(http.StatusForbidden, codeForbidden, "DELETE", path, nil)
	})
}
