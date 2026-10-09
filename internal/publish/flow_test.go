package publish

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

var firstSite = site{
	"index.html":               "<h1>Hello</h1>",
	"assets/app.js":            "console.log('hi')",
	".well-known/security.txt": "Contact: mailto:alex@example.com",
	"empty.txt":                "",
}

func TestFullPublishFlow(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		p := e.begin("acme-docs", firstSite)

		if p.Site.Slug != "acme-docs" || p.Version.Status != StatusPending || p.Version.FileCount != 4 || len(p.Files) != 4 {
			t.Fatalf("pending = %+v", p)
		}
		if p.Version.TotalBytes != int64(len("<h1>Hello</h1>")+len("console.log('hi')")+len("Contact: mailto:alex@example.com")) {
			t.Errorf("TotalBytes = %d", p.Version.TotalBytes)
		}
		for _, f := range p.Files {
			if f.ContentType == "" {
				t.Errorf("%s has no content type", f.Path)
			}
		}
		manifest, err := e.svc.LiveManifest(ctx, e.alex, "acme-docs")
		if err != nil || manifest.VersionID != "" || len(manifest.Files) != 0 {
			t.Errorf("manifest before finalize = %+v, %v", manifest, err)
		}
		if _, err := e.svc.OpenLiveFile(ctx, e.alex, "acme-docs", "index.html"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("OpenLiveFile before finalize = %v, want ErrNotFound", err)
		}

		e.upload(e.alex, p, firstSite)
		result, err := e.svc.Finalize(ctx, e.alex, "acme-docs", p.Version.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Version.Live || result.Version.Status != StatusFinalized || result.Version.FinalizedAt == nil {
			t.Errorf("result = %+v", result.Version)
		}
		if result.Site.CurrentVersionID != p.Version.ID {
			t.Errorf("site points at %q, want %q", result.Site.CurrentVersionID, p.Version.ID)
		}

		manifest, err = e.svc.LiveManifest(ctx, e.alex, "acme-docs")
		if err != nil || manifest.VersionID != p.Version.ID || len(manifest.Files) != 4 {
			t.Fatalf("manifest = %+v, %v", manifest, err)
		}
		if got := e.readLive("acme-docs", "assets/app.js"); got != firstSite["assets/app.js"] {
			t.Errorf("live file = %q", got)
		}
		if got := e.readLive("acme-docs", ".well-known/security.txt"); got != firstSite[".well-known/security.txt"] {
			t.Errorf("dotfile = %q", got)
		}
		if got := e.readLive("acme-docs", "empty.txt"); got != "" {
			t.Errorf("empty file = %q", got)
		}
		for _, missing := range []string{"nope.html", "assets", "../index.html", "assets/", ""} {
			if _, err := e.svc.OpenLiveFile(ctx, e.alex, "acme-docs", missing); !errors.Is(err, workspaces.ErrNotFound) {
				t.Errorf("OpenLiveFile(%q) = %v, want ErrNotFound", missing, err)
			}
		}
		if !exists(e.versionDir(p.Site.ID, p.Version.ID)) {
			t.Error("version directory is missing")
		}
	})
}

func TestBeginDefaultsToPersonalWorkspaceAndResolvesFolder(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"index.html": "x"}
		personal, err := e.svc.Begin(ctx, BeginInput{UserID: e.alex, Files: specs(files)})
		if err != nil {
			t.Fatal(err)
		}
		if personal.Site.WorkspaceID != e.alex || personal.Site.Slug == "" {
			t.Errorf("site = %+v, want a generated slug in the personal workspace", personal.Site)
		}
		folder, err := e.sites.CreateFolder(ctx, e.alex, e.acme.Slug, "Reports")
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range []string{folder.ID, "reports"} {
			p, err := e.svc.Begin(ctx, BeginInput{UserID: e.alex, Workspace: e.acme.ID, Folder: ref, SpaMode: true, DisplayName: "Report", Files: specs(files)})
			if err != nil {
				t.Fatalf("Begin with folder %q: %v", ref, err)
			}
			if p.Site.FolderID != folder.ID || !p.Site.SpaMode || p.Site.WorkspaceID != e.acme.ID || p.Site.DisplayName != "Report" {
				t.Errorf("site = %+v", p.Site)
			}
		}
		_, err = e.svc.Begin(ctx, BeginInput{UserID: e.alex, Workspace: e.acme.Slug, Folder: "missing", Files: specs(files)})
		if !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("unknown folder = %v, want ErrNotFound", err)
		}
	})
}

func TestBeginRejectsBadManifestWithoutCreatingASite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		_, err := e.svc.Begin(context.Background(), BeginInput{
			UserID: e.alex, Workspace: e.acme.Slug, Slug: "ghost", Files: []FileSpec{{Path: "../x", Size: 1, SHA256: hashOf("x")}},
		})
		if err == nil {
			t.Fatal("bad manifest accepted")
		}
		if _, err := e.sites.GetSite(context.Background(), e.alex, "ghost"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("a site was created for a rejected publish: %v", err)
		}
	})
}

func TestBeginRejectsTakenSlug(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishNew("taken-site", site{"index.html": "x"})
		_, err := e.svc.Begin(context.Background(), BeginInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "taken-site", Files: specs(site{"a": "b"})})
		if !errors.Is(err, sites.ErrSlugTaken) {
			t.Errorf("error = %v, want ErrSlugTaken", err)
		}
	})
}

func TestUploadChecksContent(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"index.html": "hello world"}
		p := e.begin("checks", files)
		upload := func(path, body string) error {
			_, err := e.svc.Upload(ctx, e.alex, "checks", p.Version.ID, path, strings.NewReader(body))
			return err
		}
		if err := upload("index.html", "jello world"); !isErr(err, ErrHashMismatch) {
			t.Errorf("same-length wrong content = %v, want ErrHashMismatch", err)
		}
		if err := upload("index.html", "hello world!"); !isErr(err, ErrSizeMismatch) {
			t.Errorf("longer body = %v, want ErrSizeMismatch", err)
		}
		if err := upload("index.html", "hello"); !isErr(err, ErrSizeMismatch) {
			t.Errorf("shorter body = %v, want ErrSizeMismatch", err)
		}
		if err := upload("other.html", "hello world"); !isErr(err, ErrFileNotInList) {
			t.Errorf("path outside the manifest = %v, want ErrFileNotInList", err)
		}
		if err := upload("../index.html", "hello world"); err == nil {
			t.Error("traversal path accepted on upload")
		}
		if exists(e.versionDir(p.Site.ID, p.Version.ID)) {
			t.Error("rejected uploads created version files")
		}
		if err := upload("index.html", "hello world"); err != nil {
			t.Errorf("good upload: %v", err)
		}
		if err := upload("index.html", "hello world"); err != nil {
			t.Errorf("re-upload of the same file: %v", err)
		}
	})
}

func TestFinalizeListsMissingFiles(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"a.html": "aaa", "b.html": "bbb", "c/d.html": "ddd"}
		p := e.begin("partial", files)
		if _, err := e.svc.Upload(ctx, e.alex, "partial", p.Version.ID, "a.html", strings.NewReader("aaa")); err != nil {
			t.Fatal(err)
		}
		_, err := e.svc.Finalize(ctx, e.alex, "partial", p.Version.ID)
		var missing *MissingFilesError
		if !errors.As(err, &missing) {
			t.Fatalf("Finalize = %v, want MissingFilesError", err)
		}
		if strings.Join(missing.Paths, ",") != "b.html,c/d.html" {
			t.Errorf("missing = %v", missing.Paths)
		}
		got, err := e.sites.GetSite(ctx, e.alex, "partial")
		if err != nil || got.CurrentVersionID != "" {
			t.Errorf("a half-uploaded version went live: %+v, %v", got, err)
		}
		e.upload(e.alex, p, files)
		if _, err := e.svc.Finalize(ctx, e.alex, "partial", p.Version.ID); err != nil {
			t.Errorf("Finalize after re-upload: %v", err)
		}
	})
}

func TestFinalizeCatchesDamagedFiles(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"a.html": "aaa", "b.html": "bbb"}
		p := e.begin("damaged", files)
		e.upload(e.alex, p, files)
		damage(t, e.versionDir(p.Site.ID, p.Version.ID), "a.html", "xxx")
		damage(t, e.versionDir(p.Site.ID, p.Version.ID), "b.html", "bb")
		_, err := e.svc.Finalize(ctx, e.alex, "damaged", p.Version.ID)
		var missing *MissingFilesError
		if !errors.As(err, &missing) || strings.Join(missing.Paths, ",") != "a.html,b.html" {
			t.Fatalf("Finalize = %v, want both files reported", err)
		}
	})
}

func TestFinalizeIsIdempotent(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"index.html": "one"}
		first := e.publishNew("idem", files)
		again, err := e.svc.Finalize(ctx, e.alex, "idem", first.Version.ID)
		if err != nil {
			t.Fatal(err)
		}
		if again.Version.ID != first.Version.ID || !again.Version.FinalizedAt.Equal(*first.Version.FinalizedAt) || !again.Version.Live {
			t.Errorf("second finalize = %+v, first = %+v", again.Version, first.Version)
		}
		second := e.republish("idem", first.Version.ID, site{"index.html": "two"})
		stale, err := e.svc.Finalize(ctx, e.alex, "idem", first.Version.ID)
		if err != nil {
			t.Fatalf("finalizing an older finalized version again failed: %v", err)
		}
		if stale.Version.Live || stale.Site.CurrentVersionID != second.Version.ID {
			t.Errorf("repeat finalize moved the pointer: %+v", stale)
		}
	})
}

func TestFinalizedVersionRejectsUploads(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		files := site{"index.html": "one"}
		first := e.publishNew("frozen", files)
		_, err := e.svc.Upload(context.Background(), e.alex, "frozen", first.Version.ID, "index.html", strings.NewReader("one"))
		if !isErr(err, ErrVersionFinalized) {
			t.Errorf("Upload to a finalized version = %v, want ErrVersionFinalized", err)
		}
		if got := e.readLive("frozen", "index.html"); got != "one" {
			t.Errorf("live content changed: %q", got)
		}
	})
}

func TestVersionBelongsToItsSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		first := e.publishNew("site-one", site{"a": "1"})
		e.publishNew("site-two", site{"a": "2"})
		if _, err := e.svc.Finalize(ctx, e.alex, "site-two", first.Version.ID); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("Finalize across sites = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.Restore(ctx, e.alex, "site-two", first.Version.ID); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("Restore across sites = %v, want ErrNotFound", err)
		}
		if err := e.svc.DeleteVersion(ctx, e.alex, "site-two", first.Version.ID); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("DeleteVersion across sites = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.Upload(ctx, e.alex, "site-two", first.Version.ID, "a", strings.NewReader("1")); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("Upload across sites = %v, want ErrNotFound", err)
		}
	})
}

func TestDeletingASiteRemovesItsDirectory(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		first := e.publishNew("doomed", site{"index.html": "x"})
		keeper := e.publishNew("keeper", site{"index.html": "y"})
		if !exists(e.siteDir(first.Site.ID)) {
			t.Fatal("site directory missing before delete")
		}
		if err := e.sites.DeleteSite(context.Background(), e.alex, "doomed"); err != nil {
			t.Fatal(err)
		}
		if exists(e.siteDir(first.Site.ID)) {
			t.Error("site directory survived the delete")
		}
		if !exists(e.siteDir(keeper.Site.ID)) {
			t.Error("another site's directory was removed")
		}
		if e.versionCount(first.Site.ID) != 0 {
			t.Error("version rows survived the delete")
		}
	})
}
