package publish

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func expectLimit(t *testing.T, err error, want PendingLimit) {
	t.Helper()
	var limit *PendingLimitError
	if !errors.As(err, &limit) || limit.Limit != want {
		t.Fatalf("error = %v, want a %s pending limit", err, want)
	}
}

func TestPendingVersionsAreCappedPerSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		live := e.publishNew("busy", site{"index.html": "ok"})
		for range MaxPendingPerSite {
			e.next("busy", live.Version.ID, site{"index.html": "next"})
		}
		_, err := e.svc.NewVersion(context.Background(), NewVersionInput{
			UserID: e.alex, Site: "busy", BaseVersionID: live.Version.ID, Files: specs(site{"index.html": "again"}),
		})
		expectLimit(t, err, LimitSite)
	})
}

func TestPendingVersionsAreCappedPerUser(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		for i := range MaxPendingPerUser {
			e.begin(fmt.Sprintf("site-%d", i), site{"index.html": "ok"})
		}
		_, err := e.svc.Begin(context.Background(), BeginInput{
			UserID: e.alex, Workspace: e.acme.Slug, Slug: "one-too-many", Files: specs(site{"index.html": "ok"}),
		})
		expectLimit(t, err, LimitUser)
		var shells int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sites WHERE slug = 'one-too-many'").Scan(&shells); err != nil || shells != 0 {
			t.Errorf("the refused publish left %d site rows, err %v", shells, err)
		}
		_, err = e.svc.Begin(context.Background(), BeginInput{UserID: e.sam, Slug: "sams-own", Files: specs(site{"index.html": "ok"})})
		if err != nil {
			t.Errorf("another user was blocked by alex's pending versions: %v", err)
		}
	})
}

func TestPendingBytesAreCappedAcrossTheInstance(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.set(`{"limits.maxPendingBytes": 10}`)
		e.begin("first", site{"a": "123456"})
		_, err := e.svc.Begin(context.Background(), BeginInput{
			UserID: e.sam, Slug: "second", Files: specs(site{"a": "12345"}),
		})
		expectLimit(t, err, LimitBytes)
		if _, err := e.svc.Begin(context.Background(), BeginInput{UserID: e.sam, Slug: "third", Files: specs(site{"a": "1234"})}); err != nil {
			t.Errorf("a manifest that fits was refused: %v", err)
		}
	})
}

func TestPendingBytesCapCanBeDisabled(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.set(`{"limits.maxPendingBytes": 0}`)
		e.begin("first", site{"a": "123456"})
		e.begin("second", site{"a": "123456"})
	})
}

func TestFinalizeFreesPendingBudget(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.set(`{"limits.maxPendingBytes": 6}`)
		files := site{"a": "123456"}
		p := e.begin("first", files)
		_, err := e.svc.Begin(context.Background(), BeginInput{UserID: e.sam, Slug: "second", Files: specs(files)})
		expectLimit(t, err, LimitBytes)
		e.upload(e.alex, p, files)
		if _, err := e.svc.Finalize(context.Background(), e.alex, "first", p.Version.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Begin(context.Background(), BeginInput{UserID: e.sam, Slug: "second", Files: specs(files)}); err != nil {
			t.Errorf("finalize did not free the budget: %v", err)
		}
	})
}

func TestExpiredPendingVersionsFreeBudgetWithoutWaitingForMaintenance(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		live := e.publishNew("stale", site{"index.html": "ok"})
		for range MaxPendingPerSite {
			e.next("stale", live.Version.ID, site{"index.html": "next"})
		}
		e.clock.Advance(PendingTTL + time.Minute)
		if _, err := e.svc.NewVersion(context.Background(), NewVersionInput{
			UserID: e.alex, Site: "stale", BaseVersionID: live.Version.ID, Files: specs(site{"index.html": "fresh"}),
		}); err != nil {
			t.Errorf("expired pending versions still blocked a new one: %v", err)
		}
	})
}

func TestParallelCreatesCannotExceedTheCaps(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		live := e.publishNew("parallel", site{"index.html": "ok"})
		var wg sync.WaitGroup
		var mu sync.Mutex
		accepted, refused := 0, 0
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := e.svc.NewVersion(context.Background(), NewVersionInput{
					UserID: e.alex, Site: "parallel", BaseVersionID: live.Version.ID, Files: specs(site{"index.html": "next"}),
				})
				mu.Lock()
				defer mu.Unlock()
				var limit *PendingLimitError
				switch {
				case err == nil:
					accepted++
				case errors.As(err, &limit):
					refused++
				default:
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
		if accepted != MaxPendingPerSite || refused != 20-MaxPendingPerSite {
			t.Errorf("accepted %d and refused %d, want %d and %d", accepted, refused, MaxPendingPerSite, 20-MaxPendingPerSite)
		}
	})
}

func TestParallelCreatesCannotExceedThePendingByteCap(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.set(`{"limits.maxPendingBytes": 30}`)
		var wg sync.WaitGroup
		var mu sync.Mutex
		accepted := 0
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := e.svc.Begin(context.Background(), BeginInput{
					UserID: e.sam, Slug: fmt.Sprintf("race-%d", i), Files: specs(site{"a": "0123456789"}),
				})
				if err == nil {
					mu.Lock()
					accepted++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if accepted != 3 {
			t.Errorf("accepted %d manifests of 10 bytes under a 30 byte cap, want 3", accepted)
		}
	})
}
