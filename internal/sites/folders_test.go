package sites

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func TestFolderNamesAreUniquePerWorkspaceIgnoringCase(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		e.folder("Reports")
		for _, name := range []string{"Reports", "reports", "REPORTS", " Reports "} {
			if _, err := e.svc.CreateFolder(ctx, e.alex, e.acme.Slug, name); !errors.Is(err, ErrFolderNameTaken) {
				t.Errorf("CreateFolder(%q) = %v, want ErrFolderNameTaken", name, err)
			}
		}
		if _, err := e.svc.CreateFolder(ctx, e.alex, e.acme.Slug, "Éclair"); err != nil {
			t.Fatalf("CreateFolder Éclair: %v", err)
		}
		if _, err := e.svc.CreateFolder(ctx, e.alex, e.acme.Slug, "éclair"); !errors.Is(err, ErrFolderNameTaken) {
			t.Errorf("accented duplicate = %v, want ErrFolderNameTaken", err)
		}
		other, err := e.ws.CreateShared(ctx, e.sam, "Sam team", "")
		if err != nil {
			t.Fatalf("CreateShared: %v", err)
		}
		if _, err := e.svc.CreateFolder(ctx, e.sam, other.Workspace.Slug, "reports"); err != nil {
			t.Errorf("same name in another workspace: %v", err)
		}
	})
}

func TestFolderValidationAndRename(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		for _, bad := range []string{"", "  ", strings.Repeat("a", MaxFolderNameLength+1), "a\nb"} {
			if _, err := e.svc.CreateFolder(ctx, e.alex, e.acme.Slug, bad); !isValidation(err) {
				t.Errorf("CreateFolder(%q) = %v, want validation error", bad, err)
			}
		}
		reports := e.folder("Reports")
		archive := e.folder("Archive")
		renamed, err := e.svc.RenameFolder(ctx, e.alex, e.acme.Slug, reports.ID, "Reports 2026")
		if err != nil || renamed.Name != "Reports 2026" {
			t.Fatalf("RenameFolder = %+v, %v", renamed, err)
		}
		if _, err := e.svc.RenameFolder(ctx, e.alex, e.acme.Slug, reports.ID, "ARCHIVE"); !errors.Is(err, ErrFolderNameTaken) {
			t.Errorf("rename onto another folder = %v, want ErrFolderNameTaken", err)
		}
		if _, err := e.svc.RenameFolder(ctx, e.alex, e.acme.Slug, archive.ID, "archive"); err != nil {
			t.Errorf("changing only the case of its own name: %v", err)
		}
		folders, err := e.svc.ListFolders(ctx, e.alex, e.acme.Slug)
		if err != nil || len(folders) != 2 || folders[0].Name != "archive" || folders[1].Name != "Reports 2026" {
			t.Errorf("ListFolders = %+v, %v", folders, err)
		}
	})
}

func TestDeletingAFolderMovesItsSitesToUnfiled(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		keep := e.folder("Keep")
		e.site("one", reports.ID)
		e.site("two", reports.ID)
		e.site("three", keep.ID)
		e.grant("sam@example.com", "read", "folder", reports.ID)

		if err := e.svc.DeleteFolder(ctx, e.alex, e.acme.Slug, reports.ID, true); err != nil {
			t.Fatalf("DeleteFolder: %v", err)
		}
		if got := e.slugs(e.alex, Filter{Folder: FolderNone}); !sameSet(got, []string{"one", "two"}) {
			t.Errorf("unfiled after delete = %v", got)
		}
		if got := e.slugs(e.alex, Filter{}); len(got) != 3 {
			t.Errorf("sites after delete = %v, want all three", got)
		}
		if got := e.slugs(e.sam, Filter{}); len(got) != 0 {
			t.Errorf("folder-scoped reader still sees %v after the folder was deleted", got)
		}
		site, err := e.svc.GetSite(ctx, e.alex, "one")
		if err != nil || site.FolderID != "" {
			t.Errorf("site after folder delete = %+v, %v", site, err)
		}
		if err := e.svc.DeleteFolder(ctx, e.alex, e.acme.Slug, reports.ID, false); err == nil {
			t.Error("deleting a folder twice succeeded")
		}
	})
}

func TestFolderScopedWritersCannotRenameOrDeleteTheirFolder(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		e.site("one", reports.ID)
		e.grant("sam@example.com", "write", "folder", reports.ID)

		if _, err := e.svc.RenameFolder(ctx, e.sam, e.acme.Slug, reports.ID, "Taken over"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("RenameFolder = %v, want ErrForbidden", err)
		}
		if err := e.svc.DeleteFolder(ctx, e.sam, e.acme.Slug, reports.ID, true); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("DeleteFolder = %v, want ErrForbidden", err)
		}
		folders, err := e.svc.ListFolders(ctx, e.alex, e.acme.Slug)
		if err != nil || len(folders) != 1 || folders[0].Name != "Reports" {
			t.Errorf("folders after refused changes = %+v, %v", folders, err)
		}
		if got := e.slugs(e.sam, Filter{}); len(got) != 1 {
			t.Errorf("the folder writer lost access: %v", got)
		}
	})
}

func TestWorkspaceWritersCanRenameButNotDelete(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		e.grant("sam@example.com", "write", "workspace", "")
		if _, err := e.svc.RenameFolder(ctx, e.sam, e.acme.Slug, reports.ID, "Reports 2026"); err != nil {
			t.Errorf("workspace writer RenameFolder: %v", err)
		}
		if err := e.svc.DeleteFolder(ctx, e.sam, e.acme.Slug, reports.ID, false); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("workspace writer DeleteFolder = %v, want ErrForbidden", err)
		}
	})
}

func TestFoldersAreHiddenFromUsersWithoutAccessToThem(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		other := e.folder("Other")
		e.grant("sam@example.com", "write", "folder", other.ID)
		if err := e.svc.DeleteFolder(ctx, e.sam, e.acme.Slug, reports.ID, false); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("DeleteFolder of a folder without access = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.RenameFolder(ctx, e.sam, e.acme.Slug, reports.ID, "x"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("RenameFolder of a folder without access = %v, want ErrNotFound", err)
		}
	})
}

func TestDeletingAFolderWithGrantsNeedsForce(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		e.site("one", reports.ID)
		e.grant("sam@example.com", "write", "folder", reports.ID)

		if err := e.svc.DeleteFolder(ctx, e.alex, e.acme.Slug, reports.ID, false); !errors.Is(err, ErrFolderHasGrants) {
			t.Fatalf("DeleteFolder = %v, want ErrFolderHasGrants", err)
		}
		if got := e.slugs(e.sam, Filter{}); len(got) != 1 {
			t.Errorf("a refused delete changed access: %v", got)
		}
		if err := e.svc.DeleteFolder(ctx, e.alex, e.acme.Slug, reports.ID, true); err != nil {
			t.Fatalf("forced DeleteFolder: %v", err)
		}
	})
}

func TestDeletingAFolderWithoutGrantsNeedsNoForce(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		reports := e.folder("Reports")
		if err := e.svc.DeleteFolder(context.Background(), e.alex, e.acme.Slug, reports.ID, false); err != nil {
			t.Errorf("DeleteFolder: %v", err)
		}
	})
}
