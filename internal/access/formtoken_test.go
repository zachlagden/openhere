package access

import (
	"context"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestFormTokenBindsSiteEpochAndClient(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		token, err := e.svc.FormToken(ctx, e.site.ID, 1, "203.0.113.5")
		if err != nil {
			t.Fatal(err)
		}
		check := func(siteID string, epoch int64, client, tok string) bool {
			ok, err := e.svc.VerifyFormToken(ctx, siteID, epoch, client, tok)
			if err != nil {
				t.Fatal(err)
			}
			return ok
		}
		if !check(e.site.ID, 1, "203.0.113.5", token) {
			t.Error("a fresh token was refused")
		}
		if check(e.other.ID, 1, "203.0.113.5", token) {
			t.Error("a token worked for another site")
		}
		if check(e.site.ID, 2, "203.0.113.5", token) {
			t.Error("a token survived an epoch change")
		}
		if check(e.site.ID, 1, "198.51.100.7", token) {
			t.Error("a token worked from another client")
		}
		for _, bad := range []string{"", "x", "!!!", token + "A"} {
			if check(e.site.ID, 1, "203.0.113.5", bad) {
				t.Errorf("token %q accepted", bad)
			}
		}
	})
}

func TestFormTokenLastsBetweenOneAndTwoHours(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		token, err := e.svc.FormToken(ctx, e.site.ID, 0, "client")
		if err != nil {
			t.Fatal(err)
		}
		e.clock.Advance(formWindow)
		if ok, _ := e.svc.VerifyFormToken(ctx, e.site.ID, 0, "client", token); !ok {
			t.Error("a token was refused one window later")
		}
		e.clock.Advance(formWindow)
		if ok, _ := e.svc.VerifyFormToken(ctx, e.site.ID, 0, "client", token); ok {
			t.Error("a token was accepted two windows later")
		}
	})
}
