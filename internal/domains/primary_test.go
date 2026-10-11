package domains

import (
	"context"
	"sync"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
)

func (e *env) primaries(siteID string) []string {
	e.t.Helper()
	list, err := e.svc.List(context.Background(), siteID)
	if err != nil {
		e.t.Fatal(err)
	}
	var names []string
	for _, d := range list {
		if d.IsPrimary {
			names = append(names, d.Name)
		}
	}
	return names
}

func TestSetPrimaryMovesTheFlag(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		a, b := e.verified(siteID, "a.example.org"), e.verified(siteID, "b.example.org")
		ctx := context.Background()
		got, err := e.svc.SetPrimary(ctx, siteID, a.ID, true)
		if err != nil || !got.IsPrimary {
			t.Fatalf("SetPrimary(a) = %+v, %v", got, err)
		}
		if _, err := e.svc.SetPrimary(ctx, siteID, a.ID, true); err != nil {
			t.Errorf("setting the primary again = %v", err)
		}
		if _, err := e.svc.SetPrimary(ctx, siteID, b.ID, true); err != nil {
			t.Fatal(err)
		}
		if names := e.primaries(siteID); len(names) != 1 || names[0] != "b.example.org" {
			t.Errorf("primaries = %v, want only b.example.org", names)
		}
		cleared, err := e.svc.SetPrimary(ctx, siteID, b.ID, false)
		if err != nil || cleared.IsPrimary {
			t.Errorf("clearing = %+v, %v", cleared, err)
		}
		if names := e.primaries(siteID); len(names) != 0 {
			t.Errorf("primaries after clearing = %v", names)
		}
	})
}

func TestSetPrimaryRejectsUnverifiedAndForeignDomains(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		mine, theirs := e.site("launch"), e.site("guides")
		ctx := context.Background()
		pending := e.claim(mine, "pending.example.org")
		foreign := e.verified(theirs, "other.example.org")
		if _, err := e.svc.SetPrimary(ctx, mine, pending.ID, true); !isErr(err, ErrNotVerified) {
			t.Errorf("unverified primary = %v, want ErrNotVerified", err)
		}
		if _, err := e.svc.SetPrimary(ctx, mine, foreign.ID, true); !isErr(err, ErrNotFound) {
			t.Errorf("foreign primary = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.SetPrimary(ctx, mine, "missing", false); !isErr(err, ErrNotFound) {
			t.Errorf("unknown id = %v, want ErrNotFound", err)
		}
	})
}

func TestARejectedPrimaryChangeKeepsTheOldPrimary(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		ctx := context.Background()
		a := e.verified(siteID, "a.example.org")
		pending := e.claim(siteID, "pending.example.org")
		if _, err := e.svc.SetPrimary(ctx, siteID, a.ID, true); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.SetPrimary(ctx, siteID, pending.ID, true); !isErr(err, ErrNotVerified) {
			t.Fatalf("SetPrimary = %v", err)
		}
		if names := e.primaries(siteID); len(names) != 1 || names[0] != "a.example.org" {
			t.Errorf("primaries = %v, want a.example.org to stay", names)
		}
	})
}

func TestConcurrentPrimaryChangesLeaveExactlyOnePrimary(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		ids := []string{e.verified(siteID, "a.example.org").ID, e.verified(siteID, "b.example.org").ID, e.verified(siteID, "c.example.org").ID}
		var wg sync.WaitGroup
		for i := range 18 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := e.svc.SetPrimary(context.Background(), siteID, ids[i%len(ids)], true)
				if err != nil && !store.IsUniqueViolation(err) {
					t.Errorf("SetPrimary: %v", err)
				}
			}()
		}
		wg.Wait()
		if names := e.primaries(siteID); len(names) != 1 {
			t.Errorf("primaries = %v, want exactly one", names)
		}
	})
}

func TestTheDatabaseRefusesTwoPrimaries(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		a, b := e.verified(siteID, "a.example.org"), e.verified(siteID, "b.example.org")
		ctx := context.Background()
		set := "UPDATE site_domains SET is_primary = TRUE, primary_site_id = $1 WHERE id = $2"
		if _, err := e.db.ExecContext(ctx, set, siteID, a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.ExecContext(ctx, set, siteID, b.ID); !store.IsUniqueViolation(err) {
			t.Errorf("second primary error = %v, want a unique violation", err)
		}
	})
}

func TestTheDatabaseRefusesAnUnverifiedPrimary(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		pending := e.claim(siteID, "pending.example.org")
		_, err := e.db.ExecContext(context.Background(),
			"UPDATE site_domains SET is_primary = TRUE, primary_site_id = $1 WHERE id = $2", siteID, pending.ID)
		if err == nil {
			t.Error("the database accepted an unverified primary")
		}
	})
}

func TestDeleteRemovesOnlyTheNamedDomain(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		mine, theirs := e.site("launch"), e.site("guides")
		a := e.verified(mine, "a.example.org")
		foreign := e.verified(theirs, "b.example.org")
		ctx := context.Background()
		if err := e.svc.Delete(ctx, mine, foreign.ID); !isErr(err, ErrNotFound) {
			t.Errorf("deleting through the wrong site = %v, want ErrNotFound", err)
		}
		if err := e.svc.Delete(ctx, mine, a.ID); err != nil {
			t.Fatal(err)
		}
		if err := e.svc.Delete(ctx, mine, a.ID); !isErr(err, ErrNotFound) {
			t.Errorf("second delete = %v, want ErrNotFound", err)
		}
		if !e.serves("b.example.org") || e.serves("a.example.org") {
			t.Error("delete changed the wrong domain")
		}
	})
}
