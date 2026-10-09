package sites

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
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

		if err := e.svc.DeleteFolder(ctx, e.alex, e.acme.Slug, reports.ID); err != nil {
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
		if err := e.svc.DeleteFolder(ctx, e.alex, e.acme.Slug, reports.ID); err == nil {
			t.Error("deleting a folder twice succeeded")
		}
	})
}
