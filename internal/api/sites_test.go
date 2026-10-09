package api

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestSiteLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")

		generated := alex.createSite("acme", createSiteRequest{DisplayName: "Spring launch"})
		if generated.Slug == "" || generated.DisplayName != "Spring launch" || generated.Role != "admin" || generated.WorkspaceSlug != "acme" ||
			generated.FolderID != nil || generated.CreatedBy == nil || *generated.CreatedBy != alex.userID {
			t.Errorf("generated site = %+v", generated)
		}
		site := alex.createSite("acme", createSiteRequest{Slug: "docs-site", Description: "Product docs", FolderID: reports.ID})
		if site.DisplayName != "docs-site" || site.FolderID == nil || *site.FolderID != reports.ID {
			t.Errorf("site = %+v", site)
		}

		alex.expectError(http.StatusConflict, codeSlugTaken, "POST", "/api/v1/workspaces/acme/sites", createSiteRequest{Slug: "docs-site"})
		for _, bad := range []string{"api", "Bad_Slug", "-x"} {
			alex.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/workspaces/acme/sites", createSiteRequest{Slug: bad})
		}
		alex.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/workspaces/acme/sites", createSiteRequest{Slug: "ghost", FolderID: "missing"})
		alex.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/workspaces/nowhere/sites", createSiteRequest{Slug: "ghost"})

		got := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/docs-site", nil))
		if got.Site.ID != site.ID || got.Site.Description != "Product docs" {
			t.Errorf("get = %+v", got.Site)
		}

		patched := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/docs-site", `{"displayName":"Docs","description":"","folderId":null}`))
		if patched.Site.DisplayName != "Docs" || patched.Site.Description != "" || patched.Site.FolderID != nil || patched.Site.Slug != "docs-site" {
			t.Errorf("patched = %+v", patched.Site)
		}
		refiled := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/docs-site", `{"folderId":"`+reports.ID+`"}`))
		if refiled.Site.FolderID == nil || *refiled.Site.FolderID != reports.ID || refiled.Site.DisplayName != "Docs" {
			t.Errorf("refiled = %+v", refiled.Site)
		}
		untouched := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/docs-site", `{"description":"Updated"}`))
		if untouched.Site.FolderID == nil || untouched.Site.Description != "Updated" {
			t.Errorf("folder changed by a patch that omitted it: %+v", untouched.Site)
		}
		alex.expectError(http.StatusBadRequest, codeInvalidJSON, "PATCH", "/api/v1/sites/docs-site", `{"slug":"renamed"}`)
		alex.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/sites/docs-site", `{"displayName":" "}`)
		alex.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/sites/docs-site", `{"folderId":"missing"}`)

		alex.expect(http.StatusNoContent, "DELETE", "/api/v1/sites/docs-site", nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/docs-site", nil)
		alex.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/sites/docs-site", nil)
	})
}

func TestSiteListFiltersAndPaging(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")
		for _, slug := range []string{"q1-report", "q2-report", "launch"} {
			folder := ""
			if slug != "launch" {
				folder = reports.ID
			}
			h.clock.Advance(1_000_000_000)
			alex.createSite("acme", createSiteRequest{Slug: slug, DisplayName: "Page " + slug, FolderID: folder})
		}
		h.clock.Advance(1_000_000_000)
		alex.createSite("acme", createSiteRequest{Slug: "notes", DisplayName: "Reading Notes"})

		requireSlugs(t, "all", alex.siteSlugs(""), []string{"q1-report", "q2-report", "launch", "notes"})
		requireSlugs(t, "folder", alex.siteSlugs("?folder="+url.QueryEscape(reports.ID)), []string{"q1-report", "q2-report"})
		requireSlugs(t, "unfiled", alex.siteSlugs("?folder=none"), []string{"launch", "notes"})
		requireSlugs(t, "workspace and search", alex.siteSlugs("?workspace=acme&q=REPORT"), []string{"q1-report", "q2-report"})
		requireSlugs(t, "search display name", alex.siteSlugs("?q=reading"), []string{"notes"})
		requireSlugs(t, "no match", alex.siteSlugs("?q=zzz"), nil)

		var seen []string
		query := "?limit=3"
		for pages := 0; pages < 5; pages++ {
			list := decodeBody[siteListResponse](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites"+query, nil))
			for _, site := range list.Sites {
				seen = append(seen, site.Slug)
			}
			if list.NextCursor == nil {
				break
			}
			query = "?limit=3&cursor=" + url.QueryEscape(*list.NextCursor)
		}
		requireSlugs(t, "paged", seen, []string{"q1-report", "q2-report", "launch", "notes"})

		alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites?workspace=nowhere", nil)
		for _, bad := range []string{"limit=0", "limit=abc", "limit=" + strconv.Itoa(1000), "cursor=%21%21"} {
			alex.expectError(http.StatusBadRequest, codeValidationFailed, "GET", "/api/v1/sites?"+bad, nil)
		}
	})
}

func TestScopedMemberSeesOnlyCoveredSites(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		alex := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		alex.createWorkspace("Acme Ltd", "acme")
		reports := alex.createAcmeFolder("Reports")
		archive := alex.createAcmeFolder("Archive")
		alex.createSite("acme", createSiteRequest{Slug: "q1-report", FolderID: reports.ID})
		alex.createSite("acme", createSiteRequest{Slug: "old-report", FolderID: archive.ID})
		alex.createSite("acme", createSiteRequest{Slug: "launch"})
		alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read", ScopeType: "folder", ScopeID: reports.ID})

		requireSlugs(t, "sam's sites", sam.siteSlugs(""), []string{"q1-report"})
		requireSlugs(t, "sam's search", sam.siteSlugs("?q=report"), []string{"q1-report"})
		workspaces := decodeBody[workspaceListResponse](t, sam.expect(http.StatusOK, "GET", "/api/v1/workspaces", nil))
		if len(workspaces.Workspaces) != 2 || !workspaces.Workspaces[1].ScopedOnly || workspaces.Workspaces[1].Role != "read" {
			t.Errorf("sam's workspaces = %+v", workspaces.Workspaces)
		}
		folders := decodeBody[folderListResponse](t, sam.expect(http.StatusOK, "GET", "/api/v1/workspaces/acme/folders", nil))
		if len(folders.Folders) != 1 || folders.Folders[0].ID != reports.ID {
			t.Errorf("sam's folders = %+v", folders.Folders)
		}

		sam.expect(http.StatusOK, "GET", "/api/v1/sites/q1-report", nil)
		for _, hidden := range []string{"old-report", "launch"} {
			sam.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/sites/"+hidden, nil)
			sam.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/sites/"+hidden, `{"description":"x"}`)
			sam.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/sites/"+hidden, nil)
		}
		sam.expectError(http.StatusForbidden, codeForbidden, "PATCH", "/api/v1/sites/q1-report", `{"description":"x"}`)
		sam.expectError(http.StatusForbidden, codeForbidden, "DELETE", "/api/v1/sites/q1-report", nil)
		sam.expectError(http.StatusForbidden, codeForbidden, "POST", "/api/v1/workspaces/acme/sites", createSiteRequest{Slug: "mine", FolderID: reports.ID})
		sam.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/workspaces/acme/sites", createSiteRequest{Slug: "mine", FolderID: archive.ID})
		sam.expectError(http.StatusForbidden, codeForbidden, "POST", "/api/v1/workspaces/acme/folders", folderRequest{Name: "Mine"})
		sam.expectError(http.StatusForbidden, codeForbidden, "PATCH", "/api/v1/workspaces/acme/folders/"+reports.ID, folderRequest{Name: "Mine"})
		sam.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/workspaces/acme/folders/"+archive.ID, folderRequest{Name: "Mine"})
	})
}
