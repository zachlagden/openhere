package publish

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestStaleBaseIsRejectedWhenStartingAVersion(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		v1 := e.publishNew("app-site", site{"index.html": "v1"})
		v2 := e.republish("app-site", v1.Version.ID, site{"index.html": "v2"})

		_, err := e.svc.NewVersion(ctx, NewVersionInput{UserID: e.alex, Site: "app-site", BaseVersionID: v1.Version.ID, Files: specs(site{"index.html": "v3"})})
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.CurrentVersionID != v2.Version.ID {
			t.Fatalf("error = %v, want a ConflictError naming %s", err, v2.Version.ID)
		}
		_, err = e.svc.NewVersion(ctx, NewVersionInput{UserID: e.alex, Site: "app-site", BaseVersionID: "not-a-version", Files: specs(site{"index.html": "v3"})})
		if !errors.As(err, &conflict) {
			t.Errorf("unknown base = %v, want a ConflictError", err)
		}
		if e.versionCount(v1.Site.ID) != 2 {
			t.Errorf("a conflicting publish left %d versions", e.versionCount(v1.Site.ID))
		}
		if _, err := e.svc.NewVersion(ctx, NewVersionInput{UserID: e.alex, Site: "app-site", BaseVersionID: v2.Version.ID, Files: specs(site{"index.html": "v3"})}); err != nil {
			t.Errorf("current base rejected: %v", err)
		}
	})
}

func TestPublishWithoutBaseOverwrites(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		v1 := e.publishNew("overwrite", site{"index.html": "v1"})
		e.republish("overwrite", v1.Version.ID, site{"index.html": "v2"})
		v3 := e.republish("overwrite", "", site{"index.html": "v3"})
		if !v3.Version.Live || e.readLive("overwrite", "index.html") != "v3" {
			t.Errorf("overwrite did not go live: %+v", v3.Version)
		}
	})
}

func TestFinalizeRechecksTheBase(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		v1 := e.publishNew("raced", site{"index.html": "v1"})
		mine := e.next("raced", v1.Version.ID, site{"index.html": "mine"})
		e.upload(e.alex, mine, site{"index.html": "mine"})
		theirs := e.republish("raced", v1.Version.ID, site{"index.html": "theirs"})

		_, err := e.svc.Finalize(ctx, e.alex, "raced", mine.Version.ID)
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.CurrentVersionID != theirs.Version.ID {
			t.Fatalf("Finalize = %v, want a ConflictError naming %s", err, theirs.Version.ID)
		}
		if got := e.readLive("raced", "index.html"); got != "theirs" {
			t.Errorf("live content = %q after a lost race", got)
		}
		loaded, err := loadVersion(ctx, db, mine.Site.ID, mine.Version.ID)
		if err != nil || loaded.Status != StatusPending {
			t.Errorf("losing version = %+v, %v; want it still pending", loaded, err)
		}
	})
}

func TestConcurrentFinalisationsWithTheSameBaseHaveOneWinner(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		base := e.publishNew("contended", site{"index.html": "base"})

		const contenders = 8
		pendings := make([]Pending, contenders)
		for i := range pendings {
			files := site{"index.html": "contender-" + string(rune('a'+i))}
			pendings[i] = e.next("contended", base.Version.ID, files)
			e.upload(e.alex, pendings[i], files)
		}

		errs := make([]error, contenders)
		var ready, done sync.WaitGroup
		start := make(chan struct{})
		ready.Add(contenders)
		done.Add(contenders)
		for i := range pendings {
			go func() {
				defer done.Done()
				ready.Done()
				<-start
				_, errs[i] = e.svc.Finalize(ctx, e.alex, "contended", pendings[i].Version.ID)
			}()
		}
		ready.Wait()
		close(start)
		done.Wait()

		winners := 0
		var winner string
		for i, err := range errs {
			var conflict *ConflictError
			switch {
			case err == nil:
				winners++
				winner = pendings[i].Version.ID
			case errors.As(err, &conflict):
			default:
				t.Errorf("contender %d: unexpected error %v", i, err)
			}
		}
		if winners != 1 {
			t.Fatalf("%d finalisations succeeded, want exactly 1", winners)
		}
		current, err := e.sites.GetSite(ctx, e.alex, "contended")
		if err != nil {
			t.Fatal(err)
		}
		if current.CurrentVersionID != winner {
			t.Errorf("site points at %s, the winner was %s", current.CurrentVersionID, winner)
		}
	})
}

func TestConcurrentFinalizeOfOneVersionIsSafe(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		files := site{"index.html": "once", "a/b.js": "twice"}
		p := e.begin("twin", files)
		e.upload(e.alex, p, files)

		const callers = 6
		errs := make([]error, callers)
		results := make([]Result, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Go(func() {
				results[i], errs[i] = e.svc.Finalize(ctx, e.alex, "twin", p.Version.ID)
			})
		}
		wg.Wait()
		for i := range callers {
			if errs[i] != nil {
				t.Errorf("caller %d: %v", i, errs[i])
				continue
			}
			if results[i].Version.ID != p.Version.ID || !results[i].Version.Live {
				t.Errorf("caller %d got %+v", i, results[i].Version)
			}
		}
	})
}

func TestConcurrentFirstPublishesToNewSitesDoNotInterfere(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		var wg sync.WaitGroup
		slugs := []string{"site-a", "site-b", "site-c", "site-d"}
		for _, slug := range slugs {
			wg.Go(func() {
				e.publishNew(slug, site{"index.html": slug})
			})
		}
		wg.Wait()
		for _, slug := range slugs {
			if got := e.readLive(slug, "index.html"); got != slug {
				t.Errorf("%s serves %q", slug, got)
			}
		}
	})
}
