package sites

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type scopedSetup struct {
	*env
	reports, archive Folder
	inReports        Site
	inArchive        Site
	unfiled          Site
}

func newScopedSetup(t *testing.T, db store.Store) scopedSetup {
	t.Helper()
	e := newEnv(t, db, Options{})
	s := scopedSetup{env: e, reports: e.folder("Reports"), archive: e.folder("Archive")}
	s.inReports = e.site("q1-report", s.reports.ID)
	s.inArchive = e.site("old-report", s.archive.ID)
	s.unfiled = e.site("loose-page", "")
	return s
}

func TestFolderScopedReaderSeesOnlyThatFolder(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		s.grant("sam@example.com", "read", "folder", s.reports.ID)

		if got := s.slugs(s.sam, Filter{}); !sameSet(got, []string{"q1-report"}) {
			t.Errorf("sam sees %v, want only q1-report", got)
		}
		if got, err := s.svc.GetSite(ctx, s.sam, "q1-report"); err != nil || got.Role != workspaces.RoleRead {
			t.Errorf("GetSite covered = %+v, %v", got, err)
		}
		for _, hidden := range []string{"old-report", "loose-page"} {
			if _, err := s.svc.GetSite(ctx, s.sam, hidden); !errors.Is(err, workspaces.ErrNotFound) {
				t.Errorf("GetSite(%s) = %v, want ErrNotFound", hidden, err)
			}
			if err := s.svc.DeleteSite(ctx, s.sam, hidden); !errors.Is(err, workspaces.ErrNotFound) {
				t.Errorf("DeleteSite(%s) = %v, want ErrNotFound", hidden, err)
			}
		}
		name := "Renamed"
		if _, err := s.svc.UpdateSite(ctx, s.sam, "q1-report", UpdateInput{DisplayName: &name}); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("reader UpdateSite = %v, want ErrForbidden", err)
		}
		if err := s.svc.DeleteSite(ctx, s.sam, "q1-report"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("reader DeleteSite = %v, want ErrForbidden", err)
		}

		folders, err := s.svc.ListFolders(ctx, s.sam, s.acme.Slug)
		if err != nil || len(folders) != 1 || folders[0].ID != s.reports.ID || folders[0].Role != workspaces.RoleRead {
			t.Errorf("ListFolders = %+v, %v", folders, err)
		}
		if _, err := s.svc.RenameFolder(ctx, s.sam, s.acme.Slug, s.archive.ID, "Nope"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("rename hidden folder = %v, want ErrNotFound", err)
		}
		if _, err := s.svc.RenameFolder(ctx, s.sam, s.acme.Slug, s.reports.ID, "Nope"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("reader rename visible folder = %v, want ErrForbidden", err)
		}
		if got := s.slugs(s.sam, Filter{Folder: s.archive.ID}); len(got) != 0 {
			t.Errorf("filtering by a hidden folder returned %v", got)
		}
		if _, err := s.svc.ListSites(ctx, s.kim, Filter{Workspace: s.acme.Slug}); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("stranger filtering by workspace = %v, want ErrNotFound", err)
		}
	})
}

func TestSiteScopedGrant(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		s.grant("sam@example.com", "write", "site", s.unfiled.ID)

		if got := s.slugs(s.sam, Filter{}); !sameSet(got, []string{"loose-page"}) {
			t.Errorf("sam sees %v, want only loose-page", got)
		}
		desc := "Edited"
		if got, err := s.svc.UpdateSite(ctx, s.sam, "loose-page", UpdateInput{Description: &desc}); err != nil || got.Description != "Edited" {
			t.Errorf("UpdateSite = %+v, %v", got, err)
		}
		if folders, err := s.svc.ListFolders(ctx, s.sam, s.acme.Slug); err != nil || len(folders) != 0 {
			t.Errorf("ListFolders = %+v, %v; want none", folders, err)
		}
		_, err := s.svc.CreateSite(ctx, CreateInput{UserID: s.sam, Workspace: s.acme.Slug, Slug: "another"})
		if !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("site-scoped writer CreateSite = %v, want ErrForbidden", err)
		}
		if _, err := s.svc.CreateFolder(ctx, s.sam, s.acme.Slug, "Mine"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("site-scoped writer CreateFolder = %v, want ErrForbidden", err)
		}
		if err := s.svc.DeleteSite(ctx, s.sam, "loose-page"); err != nil {
			t.Errorf("site-scoped writer DeleteSite: %v", err)
		}
	})
}

func TestFolderScopedWriter(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		s.grant("sam@example.com", "write", "folder", s.reports.ID)

		created, err := s.svc.CreateSite(ctx, CreateInput{UserID: s.sam, Workspace: s.acme.Slug, Slug: "q2-report", FolderID: s.reports.ID})
		if err != nil || created.Role != workspaces.RoleWrite {
			t.Fatalf("create in covered folder = %+v, %v", created, err)
		}
		_, err = s.svc.CreateSite(ctx, CreateInput{UserID: s.sam, Workspace: s.acme.Slug, Slug: "elsewhere", FolderID: s.archive.ID})
		if !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("create in hidden folder = %v, want ErrNotFound", err)
		}
		_, err = s.svc.CreateSite(ctx, CreateInput{UserID: s.sam, Workspace: s.acme.Slug, Slug: "unfiled-try"})
		if !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("create unfiled = %v, want ErrForbidden", err)
		}
		_, err = s.svc.CreateSite(ctx, CreateInput{UserID: s.sam, Workspace: s.acme.Slug, Slug: "ghost", FolderID: "missing"})
		if !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("create in a missing folder = %v, want ErrNotFound", err)
		}
		if _, err := s.svc.CreateFolder(ctx, s.sam, s.acme.Slug, "New"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("scoped writer CreateFolder = %v, want ErrForbidden", err)
		}
		if _, err := s.svc.RenameFolder(ctx, s.sam, s.acme.Slug, s.reports.ID, "Reports 2026"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("scoped writer RenameFolder = %v, want ErrForbidden", err)
		}
		if got := s.slugs(s.sam, Filter{}); !sameSet(got, []string{"q1-report", "q2-report"}) {
			t.Errorf("sam sees %v", got)
		}
	})
}

func TestMovingSitesNeedsWriteOnTheDestination(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		s.grant("sam@example.com", "write", "folder", s.reports.ID)
		s.grant("sam@example.com", "read", "folder", s.archive.ID)
		toArchive := UpdateInput{Folder: FolderChange{Set: true, ID: s.archive.ID}}
		toUnfiled := UpdateInput{Folder: FolderChange{Set: true}}
		toMissing := UpdateInput{Folder: FolderChange{Set: true, ID: "missing"}}

		if _, err := s.svc.UpdateSite(ctx, s.sam, "q1-report", toArchive); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("move to a read-only folder = %v, want ErrForbidden", err)
		}
		if _, err := s.svc.UpdateSite(ctx, s.sam, "q1-report", toUnfiled); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("move to unfiled without workspace-wide write = %v, want ErrForbidden", err)
		}
		if _, err := s.svc.UpdateSite(ctx, s.sam, "q1-report", toMissing); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("move to a missing folder = %v, want ErrNotFound", err)
		}

		moved, err := s.svc.UpdateSite(ctx, s.alex, "q1-report", toArchive)
		if err != nil || moved.FolderID != s.archive.ID {
			t.Fatalf("admin move = %+v, %v", moved, err)
		}
		unfiled, err := s.svc.UpdateSite(ctx, s.alex, "q1-report", toUnfiled)
		if err != nil || unfiled.FolderID != "" {
			t.Fatalf("admin unfile = %+v, %v", unfiled, err)
		}
		kept, err := s.svc.UpdateSite(ctx, s.alex, "q1-report", UpdateInput{})
		if err != nil || kept.FolderID != "" {
			t.Errorf("update without a folder change = %+v, %v", kept, err)
		}
	})
}

func TestUpdateSiteValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		blank, long := " ", string(make([]byte, MaxDescriptionLength+1))
		for name, in := range map[string]UpdateInput{
			"blank display name": {DisplayName: &blank},
			"long description":   {Description: &long},
		} {
			if _, err := s.svc.UpdateSite(ctx, s.alex, "q1-report", in); !isValidation(err) {
				t.Errorf("%s = %v, want validation error", name, err)
			}
		}
		name, desc := "  Quarterly report ", ""
		got, err := s.svc.UpdateSite(ctx, s.alex, "q1-report", UpdateInput{DisplayName: &name, Description: &desc})
		if err != nil || got.DisplayName != "Quarterly report" || got.Slug != "q1-report" {
			t.Errorf("UpdateSite = %+v, %v", got, err)
		}
		byID, err := s.svc.GetSite(ctx, s.alex, s.inReports.ID)
		if err != nil || byID.DisplayName != "Quarterly report" {
			t.Errorf("GetSite by id = %+v, %v", byID, err)
		}
	})
}

func TestReadersCannotWriteAndStrangersSeeNothing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		s.grant("sam@example.com", "read", "workspace", "")

		if got := s.slugs(s.sam, Filter{}); len(got) != 3 {
			t.Errorf("workspace reader sees %v", got)
		}
		if _, err := s.svc.CreateSite(ctx, CreateInput{UserID: s.sam, Workspace: s.acme.Slug, Slug: "nope"}); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("reader CreateSite = %v, want ErrForbidden", err)
		}
		if _, err := s.svc.CreateFolder(ctx, s.sam, s.acme.Slug, "Nope"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("reader CreateFolder = %v, want ErrForbidden", err)
		}
		if err := s.svc.DeleteFolder(ctx, s.sam, s.acme.Slug, s.reports.ID, false); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("reader DeleteFolder = %v, want ErrForbidden", err)
		}
		if got := s.slugs(s.kim, Filter{}); len(got) != 0 {
			t.Errorf("stranger sees %v", got)
		}
		if _, err := s.svc.GetSite(ctx, s.kim, "q1-report"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("stranger GetSite = %v, want ErrNotFound", err)
		}
		if _, err := s.svc.CreateSite(ctx, CreateInput{UserID: s.kim, Workspace: s.acme.Slug, Slug: "nope"}); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("stranger CreateSite = %v, want ErrNotFound", err)
		}
	})
}
