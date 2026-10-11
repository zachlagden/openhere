package publish

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func publishMany(e *env, slug string, count int) []Result {
	e.t.Helper()
	results := make([]Result, 0, count)
	base := ""
	for i := range count {
		files := site{"index.html": fmt.Sprintf("version %d", i+1)}
		var r Result
		if i == 0 {
			e.clock.Advance(1_000_000_000)
			r = e.publishNew(slug, files)
		} else {
			r = e.republish(slug, base, files)
		}
		base = r.Version.ID
		results = append(results, r)
	}
	return results
}

func TestRetentionPrunesOldVersionsButNeverTheLiveOne(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.set(`{"retention.keepVersions": 3}`)
		results := publishMany(e, "kept", 6)
		siteID := results[0].Site.ID

		if n := e.versionCount(siteID); n != 3 {
			t.Fatalf("%d versions kept, want 3", n)
		}
		for i, r := range results {
			wantKept := i >= 3
			if got := exists(e.versionDir(siteID, r.Version.ID)); got != wantKept {
				t.Errorf("version %d directory present = %v, want %v", i+1, got, wantKept)
			}
		}
		listed, err := e.svc.List(ctx, e.alex, "kept")
		if err != nil || len(listed) != 3 {
			t.Fatalf("List = %d versions, %v", len(listed), err)
		}
		if listed[0].ID != results[5].Version.ID || !listed[0].Live || listed[1].Live || listed[2].Live {
			t.Errorf("list order or live flag wrong: %+v", listed)
		}
		if got := e.readLive("kept", "index.html"); got != "version 6" {
			t.Errorf("live content = %q", got)
		}
	})
}

func TestPruneKeepsARestoredLiveVersionThatIsNotTheNewest(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		results := publishMany(e, "old-live", 3)
		siteID := results[0].Site.ID
		if _, err := e.svc.Restore(ctx, e.alex, "old-live", results[0].Version.ID); err != nil {
			t.Fatal(err)
		}
		e.set(`{"retention.keepVersions": 2}`)
		e.svc.prune(ctx, siteID)

		if !exists(e.versionDir(siteID, results[0].Version.ID)) {
			t.Error("the live version was pruned")
		}
		if !exists(e.versionDir(siteID, results[2].Version.ID)) {
			t.Error("the newest other version was pruned")
		}
		if exists(e.versionDir(siteID, results[1].Version.ID)) {
			t.Error("the middle version survived")
		}
		if n := e.versionCount(siteID); n != 2 {
			t.Errorf("%d versions kept, want 2", n)
		}
		if got := e.readLive("old-live", "index.html"); got != "version 1" {
			t.Errorf("live content = %q", got)
		}
	})
}

func TestRetentionOfOneKeepsOnlyTheLiveVersion(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.set(`{"retention.keepVersions": 1}`)
		results := publishMany(e, "single", 3)
		if n := e.versionCount(results[0].Site.ID); n != 1 {
			t.Errorf("%d versions kept, want 1", n)
		}
		if got := e.readLive("single", "index.html"); got != "version 3" {
			t.Errorf("live content = %q", got)
		}
	})
}

func TestRetentionIgnoresPendingVersions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.set(`{"retention.keepVersions": 2}`)
		results := publishMany(e, "pendings", 2)
		pending := e.next("pendings", results[1].Version.ID, site{"index.html": "wip"})
		e.republish("pendings", results[1].Version.ID, site{"index.html": "version 3"})
		if _, err := loadVersion(context.Background(), db, pending.Site.ID, pending.Version.ID); err != nil {
			t.Errorf("pruning removed a pending version: %v", err)
		}
	})
}

func TestRestoreFlipsThePointerWithoutUploading(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		results := publishMany(e, "restorable", 3)
		first := results[0]

		restored, err := e.svc.Restore(ctx, e.alex, "restorable", first.Version.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !restored.Version.Live || restored.Site.CurrentVersionID != first.Version.ID {
			t.Errorf("restore result = %+v", restored)
		}
		if got := e.readLive("restorable", "index.html"); got != "version 1" {
			t.Errorf("live content = %q", got)
		}
		again, err := e.svc.Restore(ctx, e.alex, "restorable", first.Version.ID)
		if err != nil || !again.Version.Live {
			t.Errorf("restoring the live version = %+v, %v", again, err)
		}
		listed, err := e.svc.List(ctx, e.alex, "restorable")
		if err != nil {
			t.Fatal(err)
		}
		live := 0
		for _, v := range listed {
			if v.Live {
				live++
				if v.ID != first.Version.ID {
					t.Errorf("wrong version marked live: %s", v.ID)
				}
			}
		}
		if live != 1 {
			t.Errorf("%d versions marked live, want 1", live)
		}
	})
}

func TestRestoreRefusesPendingAndUnknownVersions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		first := e.publishNew("refuse", site{"index.html": "one"})
		pending := e.next("refuse", first.Version.ID, site{"index.html": "two"})
		if _, err := e.svc.Restore(ctx, e.alex, "refuse", pending.Version.ID); !errors.Is(err, ErrNotFinalized) {
			t.Errorf("restore of a pending version = %v, want ErrNotFinalized", err)
		}
		if _, err := e.svc.Restore(ctx, e.alex, "refuse", "0123456789abcdef0123456789abcdef"); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("restore of an unknown version = %v, want ErrNotFound", err)
		}
		if got := e.readLive("refuse", "index.html"); got != "one" {
			t.Errorf("live content = %q", got)
		}
	})
}

func TestDeleteRefusesTheLiveVersion(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		results := publishMany(e, "protected", 2)
		live := results[1]
		if err := e.svc.DeleteVersion(ctx, e.alex, "protected", live.Version.ID); !errors.Is(err, ErrVersionLive) {
			t.Fatalf("DeleteVersion(live) = %v, want ErrVersionLive", err)
		}
		if !exists(e.versionDir(live.Site.ID, live.Version.ID)) || e.versionCount(live.Site.ID) != 2 {
			t.Error("the live version was damaged by a refused delete")
		}

		old := results[0]
		if err := e.svc.DeleteVersion(ctx, e.alex, "protected", old.Version.ID); err != nil {
			t.Fatal(err)
		}
		if exists(e.versionDir(old.Site.ID, old.Version.ID)) || e.versionCount(old.Site.ID) != 1 {
			t.Error("the deleted version is still there")
		}
		if err := e.svc.DeleteVersion(ctx, e.alex, "protected", old.Version.ID); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("second delete = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.Restore(ctx, e.alex, "protected", old.Version.ID); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("restore of a deleted version = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteAllowsPendingVersions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		files := site{"index.html": "wip"}
		p := e.begin("abandon", files)
		e.upload(e.alex, p, files)
		if err := e.svc.DeleteVersion(context.Background(), e.alex, "abandon", p.Version.ID); err != nil {
			t.Fatal(err)
		}
		if exists(e.versionDir(p.Site.ID, p.Version.ID)) {
			t.Error("the pending version directory survived")
		}
	})
}

func TestListShowsPendingAndFinalizedVersions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		first := e.publishNew("listing", site{"index.html": "one"})
		pending := e.next("listing", first.Version.ID, site{"a": "1", "b": "22"})
		listed, err := e.svc.List(context.Background(), e.alex, "listing")
		if err != nil || len(listed) != 2 {
			t.Fatalf("List = %v, %v", listed, err)
		}
		if listed[0].ID != pending.Version.ID || listed[0].Status != StatusPending || listed[0].Live ||
			listed[0].FileCount != 2 || listed[0].TotalBytes != 3 || listed[0].BaseVersionID != first.Version.ID || listed[0].FinalizedAt != nil {
			t.Errorf("pending entry = %+v", listed[0])
		}
		if listed[1].ID != first.Version.ID || !listed[1].Live || listed[1].CreatedBy != e.alex {
			t.Errorf("finalized entry = %+v", listed[1])
		}
	})
}
