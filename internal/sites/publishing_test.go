package sites

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

type recordingCleaner struct {
	removed []string
	err     error
}

func (c *recordingCleaner) RemoveSite(siteID string) error {
	c.removed = append(c.removed, siteID)
	return c.err
}

func TestDeleteSiteRemovesFilesAfterTheRow(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		cleaner := &recordingCleaner{}
		e := newEnv(t, db, Options{Files: cleaner})
		site := e.site("handbook", "")
		if err := e.svc.DeleteSite(context.Background(), e.alex, "handbook"); err != nil {
			t.Fatal(err)
		}
		if len(cleaner.removed) != 1 || cleaner.removed[0] != site.ID {
			t.Errorf("removed = %v, want [%s]", cleaner.removed, site.ID)
		}
		var rows int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sites WHERE id = $1", site.ID).Scan(&rows); err != nil || rows != 0 {
			t.Errorf("site row count = %d, %v", rows, err)
		}
	})
}

func TestDeleteSiteSucceedsWhenFileRemovalFails(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		cleaner := &recordingCleaner{err: errors.New("disk is read-only")}
		e := newEnv(t, db, Options{Files: cleaner})
		e.site("handbook", "")
		if err := e.svc.DeleteSite(context.Background(), e.alex, "handbook"); err != nil {
			t.Fatalf("DeleteSite failed because of a disk error: %v", err)
		}
		if len(cleaner.removed) != 1 {
			t.Errorf("cleaner called %d times, want 1", len(cleaner.removed))
		}
	})
}

func TestDeniedDeleteLeavesFilesAlone(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		cleaner := &recordingCleaner{}
		e := newEnv(t, db, Options{Files: cleaner})
		e.site("handbook", "")
		e.grant("sam@example.com", "read", "workspace", "")
		if err := e.svc.DeleteSite(context.Background(), e.sam, "handbook"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Fatalf("DeleteSite = %v, want ErrForbidden", err)
		}
		if err := e.svc.DeleteSite(context.Background(), e.kim, "handbook"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Fatalf("DeleteSite = %v, want ErrNotFound", err)
		}
		if len(cleaner.removed) != 0 {
			t.Errorf("files removed for a denied delete: %v", cleaner.removed)
		}
	})
}

func TestSpaModeOnCreateAndUpdate(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		created, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "webapp", SpaMode: true})
		if err != nil {
			t.Fatal(err)
		}
		if !created.SpaMode {
			t.Error("CreateSite ignored SpaMode")
		}
		plain := e.site("plain", "")
		if plain.SpaMode {
			t.Error("a site defaults to SPA mode")
		}
		off := false
		updated, err := e.svc.UpdateSite(ctx, e.alex, "webapp", UpdateInput{SpaMode: &off})
		if err != nil || updated.SpaMode {
			t.Errorf("UpdateSite = %+v, %v; want SPA mode off", updated, err)
		}
		on := true
		updated, err = e.svc.UpdateSite(ctx, e.alex, "plain", UpdateInput{SpaMode: &on})
		if err != nil || !updated.SpaMode {
			t.Errorf("UpdateSite = %+v, %v; want SPA mode on", updated, err)
		}
		reloaded, err := e.svc.GetSite(ctx, e.alex, "plain")
		if err != nil || !reloaded.SpaMode || reloaded.CurrentVersionID != "" {
			t.Errorf("GetSite = %+v, %v", reloaded, err)
		}
		untouched, err := e.svc.UpdateSite(ctx, e.alex, "plain", UpdateInput{})
		if err != nil || !untouched.SpaMode {
			t.Errorf("an update without spaMode changed it: %+v, %v", untouched, err)
		}
	})
}

func TestAuthorizeAndFindFolder(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		e.site("q1", reports.ID)
		e.grant("sam@example.com", "read", "folder", reports.ID)

		if _, err := e.svc.Authorize(ctx, e.sam, "q1", workspaces.RoleRead); err != nil {
			t.Errorf("read: %v", err)
		}
		if _, err := e.svc.Authorize(ctx, e.sam, "q1", workspaces.RoleWrite); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("write as reader = %v, want ErrForbidden", err)
		}
		if _, err := e.svc.Authorize(ctx, e.kim, "q1", workspaces.RoleRead); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("stranger = %v, want ErrNotFound", err)
		}

		for _, ref := range []string{reports.ID, "Reports", "reports", "REPORTS"} {
			found, err := e.svc.FindFolder(ctx, e.acme.ID, ref)
			if err != nil || found.ID != reports.ID {
				t.Errorf("FindFolder(%q) = %+v, %v", ref, found, err)
			}
		}
		if _, err := e.svc.FindFolder(ctx, e.acme.ID, "Missing"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("FindFolder(missing) = %v, want ErrNotFound", err)
		}
	})
}
