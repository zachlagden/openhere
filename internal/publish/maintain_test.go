package publish

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestPendingVersionsExpireAfterADay(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"index.html": "x"}
		abandoned := e.begin("abandoned", files)
		e.upload(e.alex, abandoned, files)
		kept := e.publishNew("kept-site", files)
		fresh := e.next("kept-site", kept.Version.ID, files)

		e.clock.Advance(PendingTTL - time.Minute)
		if err := e.svc.Maintain(ctx); err != nil {
			t.Fatal(err)
		}
		if e.versionCount(abandoned.Site.ID) != 1 {
			t.Fatal("a pending version was removed before it expired")
		}

		e.clock.Advance(2 * time.Minute)
		freshNow := e.next("kept-site", kept.Version.ID, files)
		if err := e.svc.Maintain(ctx); err != nil {
			t.Fatal(err)
		}
		if e.versionCount(abandoned.Site.ID) != 0 || exists(e.versionDir(abandoned.Site.ID, abandoned.Version.ID)) {
			t.Error("the expired pending version survived maintenance")
		}
		if _, err := loadVersion(ctx, db, kept.Site.ID, fresh.Version.ID); err == nil {
			t.Error("an older expired pending version survived")
		}
		if _, err := loadVersion(ctx, db, kept.Site.ID, freshNow.Version.ID); err != nil {
			t.Errorf("a young pending version was removed: %v", err)
		}
		if _, err := loadVersion(ctx, db, kept.Site.ID, kept.Version.ID); err != nil {
			t.Errorf("a finalized version was removed: %v", err)
		}
		if got := e.readLive("kept-site", "index.html"); got != "x" {
			t.Errorf("live content = %q", got)
		}
	})
}

func TestExpiredVersionsRejectUploadAndFinalize(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"index.html": "x"}
		p := e.begin("late", files)
		e.upload(e.alex, p, files)
		e.clock.Advance(PendingTTL + time.Second)
		if _, err := e.svc.Upload(ctx, e.alex, "late", p.Version.ID, "index.html", strings.NewReader("x")); !errors.Is(err, ErrVersionExpired) {
			t.Errorf("Upload = %v, want ErrVersionExpired", err)
		}
		if _, err := e.svc.Finalize(ctx, e.alex, "late", p.Version.ID); !errors.Is(err, ErrVersionExpired) {
			t.Errorf("Finalize = %v, want ErrVersionExpired", err)
		}
	})
}

func TestMaintainSweepsOrphanedDirectories(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		live := e.publishNew("sweep-me", site{"index.html": "x"})
		strayVersion := strings.Repeat("c", 32)
		strayVersionDir := e.versionDir(live.Site.ID, strayVersion)
		straySite := e.siteDir(strings.Repeat("d", 32))
		unrelated := filepath.Join(e.dataDir, "sites", "not-an-id")
		for _, dir := range []string{filepath.Join(strayVersionDir, "files"), filepath.Join(straySite, "versions", strayVersion, "files"), unrelated} {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatal(err)
			}
		}
		oldTemp := filepath.Join(e.dataDir, "tmp", "upload-crashed")
		if err := os.WriteFile(oldTemp, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
		past := e.clock.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(oldTemp, past, past); err != nil {
			t.Fatal(err)
		}

		if err := e.svc.Maintain(ctx); err != nil {
			t.Fatal(err)
		}
		if exists(strayVersionDir) {
			t.Error("an orphaned version directory survived")
		}
		if exists(straySite) {
			t.Error("an orphaned site directory survived")
		}
		if exists(oldTemp) {
			t.Error("a stale upload file survived")
		}
		if !exists(e.versionDir(live.Site.ID, live.Version.ID)) {
			t.Error("the live version directory was swept")
		}
		if !exists(unrelated) {
			t.Error("a directory that is not an id was removed")
		}
		if got := e.readLive("sweep-me", "index.html"); got != "x" {
			t.Errorf("live content = %q", got)
		}
	})
}
