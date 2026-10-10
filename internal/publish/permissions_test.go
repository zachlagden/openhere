package publish

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type actionCase struct {
	name  string
	needs workspaces.Role
	run   func(e *env, userID string, target publishTarget) error
}

type publishTarget struct {
	slug    string
	pending Pending
	live    Result
}

func actions() []actionCase {
	files := site{"index.html": "x"}
	return []actionCase{
		{"new version", workspaces.RoleWrite, func(e *env, u string, t publishTarget) error {
			_, err := e.svc.NewVersion(context.Background(), NewVersionInput{UserID: u, Site: t.slug, Files: specs(files)})
			return err
		}},
		{"upload", workspaces.RoleWrite, func(e *env, u string, t publishTarget) error {
			_, err := e.svc.Upload(context.Background(), u, t.slug, t.pending.Version.ID, "index.html", strings.NewReader("x"))
			return err
		}},
		{"finalize", workspaces.RoleWrite, func(e *env, u string, t publishTarget) error {
			_, err := e.svc.Finalize(context.Background(), u, t.slug, t.pending.Version.ID)
			return err
		}},
		{"restore", workspaces.RoleWrite, func(e *env, u string, t publishTarget) error {
			_, err := e.svc.Restore(context.Background(), u, t.slug, t.live.Version.ID)
			return err
		}},
		{"delete version", workspaces.RoleWrite, func(e *env, u string, t publishTarget) error {
			return e.svc.DeleteVersion(context.Background(), u, t.slug, t.pending.Version.ID)
		}},
		{"list versions", workspaces.RoleRead, func(e *env, u string, t publishTarget) error {
			_, err := e.svc.List(context.Background(), u, t.slug)
			return err
		}},
		{"live manifest", workspaces.RoleRead, func(e *env, u string, t publishTarget) error {
			_, err := e.svc.LiveManifest(context.Background(), u, t.slug)
			return err
		}},
		{"live file", workspaces.RoleRead, func(e *env, u string, t publishTarget) error {
			live, err := e.svc.OpenLiveFile(context.Background(), u, t.slug, "index.html")
			if err == nil {
				return live.File.Close()
			}
			return err
		}},
	}
}

func TestPermissionMatrix(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		stranger := e.user("lee@example.com")
		e.grant("sam@example.com", "read", "workspace", "")
		e.grant("kim@example.com", "write", "workspace", "")
		roles := map[string]string{"stranger": stranger, "reader": e.sam, "writer": e.kim}
		count := 0
		for _, tc := range actions() {
			for role, userID := range roles {
				count++
				slug := fmt.Sprintf("guarded-%d", count)
				live := e.publishNew(slug, site{"index.html": "x"})
				pending := e.next(slug, live.Version.ID, site{"index.html": "x"})
				e.upload(e.alex, pending, site{"index.html": "x"})
				err := tc.run(e, userID, publishTarget{slug: slug, pending: pending, live: live})
				if _, execErr := db.ExecContext(context.Background(), "DELETE FROM versions WHERE site_id = $1 AND status = 'pending'", pending.Site.ID); execErr != nil {
					t.Fatalf("clear pending versions: %v", execErr)
				}
				switch {
				case role == "stranger":
					if !errors.Is(err, workspaces.ErrNotFound) {
						t.Errorf("%s as a stranger = %v, want ErrNotFound", tc.name, err)
					}
				case role == "reader" && tc.needs == workspaces.RoleWrite:
					if !errors.Is(err, workspaces.ErrForbidden) {
						t.Errorf("%s as a reader = %v, want ErrForbidden", tc.name, err)
					}
				default:
					if err != nil {
						t.Errorf("%s as a %s = %v, want success", tc.name, role, err)
					}
				}
			}
		}
	})
}

func TestForbiddenActionsChangeNothing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		live := e.publishNew("untouched", site{"index.html": "x"})
		pending := e.next("untouched", live.Version.ID, site{"index.html": "y"})
		e.grant("sam@example.com", "read", "workspace", "")

		if _, err := e.svc.Upload(ctx, e.sam, "untouched", pending.Version.ID, "index.html", strings.NewReader("y")); !errors.Is(err, workspaces.ErrForbidden) {
			t.Fatalf("reader upload = %v", err)
		}
		if exists(e.versionDir(pending.Site.ID, pending.Version.ID)) {
			t.Error("a forbidden upload wrote files")
		}
		if err := e.svc.DeleteVersion(ctx, e.sam, "untouched", live.Version.ID); !errors.Is(err, workspaces.ErrForbidden) {
			t.Fatalf("reader delete = %v", err)
		}
		if e.versionCount(live.Site.ID) != 2 {
			t.Error("a forbidden delete removed a version")
		}
	})
}

func TestBeginPermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		reports, err := e.sites.CreateFolder(ctx, e.alex, e.acme.Slug, "Reports")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.sites.CreateFolder(ctx, e.alex, e.acme.Slug, "Private"); err != nil {
			t.Fatal(err)
		}
		files := specs(site{"index.html": "x"})
		begin := func(userID, workspace, folder string) error {
			_, err := e.svc.Begin(ctx, BeginInput{UserID: userID, Workspace: workspace, Folder: folder, Files: files})
			return err
		}

		if err := begin(e.sam, e.acme.Slug, ""); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("stranger = %v, want ErrNotFound", err)
		}
		e.grant("sam@example.com", "read", "workspace", "")
		if err := begin(e.sam, e.acme.Slug, ""); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("workspace reader = %v, want ErrForbidden", err)
		}
		if err := begin(e.sam, e.acme.Slug, "Reports"); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("workspace reader into a folder = %v, want ErrForbidden", err)
		}

		e.grant("kim@example.com", "write", "folder", reports.ID)
		if err := begin(e.kim, e.acme.Slug, "Reports"); err != nil {
			t.Errorf("folder writer into their folder = %v, want success", err)
		}
		if err := begin(e.kim, e.acme.Slug, ""); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("folder writer workspace-wide = %v, want ErrForbidden", err)
		}
		if err := begin(e.kim, e.acme.Slug, "Private"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("folder writer into another folder = %v, want ErrNotFound", err)
		}
		if err := begin(e.kim, "no-such-workspace", ""); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("unknown workspace = %v, want ErrNotFound", err)
		}
		if err := begin(e.sam, "", ""); err != nil {
			t.Errorf("personal workspace default = %v, want success", err)
		}
	})
}

func TestSiteScopedWriterCanPublishToThatSiteOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		mine := e.publishNew("only-mine", site{"index.html": "x"})
		e.publishNew("not-mine", site{"index.html": "y"})
		e.grant("sam@example.com", "write", "site", mine.Site.ID)
		files := specs(site{"index.html": "z"})
		if _, err := e.svc.NewVersion(ctx, NewVersionInput{UserID: e.sam, Site: "only-mine", Files: files}); err != nil {
			t.Errorf("publish to the granted site: %v", err)
		}
		if _, err := e.svc.NewVersion(ctx, NewVersionInput{UserID: e.sam, Site: "not-mine", Files: files}); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("publish to another site = %v, want ErrNotFound", err)
		}
	})
}
