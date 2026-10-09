package access

import (
	"context"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func (e *env) consume(siteID, token string) (string, bool) {
	e.t.Helper()
	_, userID, ok, err := e.svc.ConsumeHandoff(context.Background(), siteID, token)
	if err != nil {
		e.t.Fatalf("ConsumeHandoff: %v", err)
	}
	return userID, ok
}

func (e *env) issue(siteID, userID string) string {
	e.t.Helper()
	token, err := e.svc.IssueHandoff(context.Background(), siteID, userID)
	if err != nil {
		e.t.Fatalf("IssueHandoff: %v", err)
	}
	return token
}

func TestHandoffTokenWorksOnce(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		token := e.issue(e.site.ID, e.alex)
		userID, ok := e.consume(e.site.ID, token)
		if !ok || userID != e.alex {
			t.Fatalf("first use: user = %q, ok = %v", userID, ok)
		}
		if _, ok := e.consume(e.site.ID, token); ok {
			t.Error("a handoff token worked twice")
		}
	})
}

func TestHandoffTokenExpiresAfterSixtySeconds(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		fresh := e.issue(e.site.ID, e.alex)
		e.clock.Advance(HandoffLifetime - 1)
		if _, ok := e.consume(e.site.ID, fresh); !ok {
			t.Error("a token was refused before it expired")
		}
		stale := e.issue(e.site.ID, e.alex)
		e.clock.Advance(HandoffLifetime)
		if _, ok := e.consume(e.site.ID, stale); ok {
			t.Error("an expired token was accepted")
		}
	})
}

func TestHandoffTokenIsBoundToItsSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		for _, id := range []string{e.site.ID, e.other.ID} {
			e.update(id, ConfigUpdate{Mode: strptr("restricted")})
		}
		token := e.issue(e.site.ID, e.alex)
		if _, ok := e.consume(e.other.ID, token); ok {
			t.Error("a token for one site worked on another")
		}
		if _, ok := e.consume(e.site.ID, token); !ok {
			t.Error("a token was burned by a failed attempt on another site")
		}
	})
}

func TestHandoffNeedsRestrictedModeAndCurrentAccess(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		outsider := e.issue(e.site.ID, e.kim)
		if _, ok := e.consume(e.site.ID, outsider); ok {
			t.Error("a user without access got a cookie")
		}
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("public")})
		stale := e.issue(e.site.ID, e.alex)
		if _, ok := e.consume(e.site.ID, stale); ok {
			t.Error("a handoff worked on a public site")
		}
	})
}

func TestUnknownAndMalformedHandoffTokensFail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		for _, token := range []string{"", "x", "0000000000000000000000000000000000000000000", "' OR 1=1 --"} {
			if _, ok := e.consume(e.site.ID, token); ok {
				t.Errorf("token %q accepted", token)
			}
		}
	})
}

func TestStoredHandoffTokenIsHashed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		token := e.issue(e.site.ID, e.alex)
		var stored string
		if err := db.QueryRowContext(context.Background(), "SELECT token_hash FROM viewer_handoffs").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored == token || len(stored) != 64 {
			t.Errorf("stored token = %q", stored)
		}
	})
}
