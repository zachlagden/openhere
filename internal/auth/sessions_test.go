package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func setupUser(t *testing.T, svc *Service) User {
	t.Helper()
	user, err := svc.CreateUser(context.Background(), "alex@example.com", testPassword)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

func TestSessionLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user := setupUser(t, svc)

		token, session, err := svc.CreateSession(ctx, user.ID)
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if len(token) != 43 || session.ID == token || len(session.ID) != 64 {
			t.Errorf("token %q or session id %q has the wrong shape", token, session.ID)
		}
		var stored string
		if err := db.QueryRowContext(ctx, "SELECT id FROM sessions").Scan(&stored); err != nil {
			t.Fatalf("read session: %v", err)
		}
		if stored == token || strings.Contains(stored, token) {
			t.Error("the raw token is stored in the database")
		}

		info, err := svc.ValidateSession(ctx, token)
		if err != nil || info.User.ID != user.ID || info.Extended {
			t.Fatalf("ValidateSession = %+v, %v", info, err)
		}
		if err := svc.DeleteSession(ctx, token); err != nil {
			t.Fatalf("DeleteSession: %v", err)
		}
		if _, err := svc.ValidateSession(ctx, token); !errors.Is(err, ErrSessionInvalid) {
			t.Errorf("validate after delete = %v, want ErrSessionInvalid", err)
		}
	})
}

func TestSessionRejectsUnknownTokens(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := newTestService(t, db, newFakeClock())
		for _, token := range []string{"", "short", strings.Repeat("a", 43)} {
			if _, err := svc.ValidateSession(context.Background(), token); !errors.Is(err, ErrSessionInvalid) {
				t.Errorf("ValidateSession(%q) = %v, want ErrSessionInvalid", token, err)
			}
		}
		if err := svc.DeleteSession(context.Background(), "short"); err != nil {
			t.Errorf("DeleteSession of junk: %v", err)
		}
	})
}

func TestSessionSlidingExpiry(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		clock := newFakeClock()
		svc := newTestService(t, db, clock)
		user := setupUser(t, svc)
		token, session, err := svc.CreateSession(ctx, user.ID)
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if want := session.CreatedAt.Add(SessionTTL); !session.ExpiresAt.Equal(want) {
			t.Errorf("expires_at = %v, want %v", session.ExpiresAt, want)
		}

		clock.Advance(20 * 24 * time.Hour)
		info, err := svc.ValidateSession(ctx, token)
		if err != nil || !info.Extended {
			t.Fatalf("validate after 20 days = %+v, %v", info, err)
		}
		if want := clock.Now().Add(SessionTTL); !info.Session.ExpiresAt.Equal(want) {
			t.Errorf("slid expires_at = %v, want %v", info.Session.ExpiresAt, want)
		}

		clock.Advance(25 * 24 * time.Hour)
		if _, err := svc.ValidateSession(ctx, token); err != nil {
			t.Errorf("session should still be valid 45 days in because it slid: %v", err)
		}
	})
}

func TestSessionExpires(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		clock := newFakeClock()
		svc := newTestService(t, db, clock)
		user := setupUser(t, svc)
		token, _, err := svc.CreateSession(ctx, user.ID)
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		clock.Advance(SessionTTL + time.Second)
		if _, err := svc.ValidateSession(ctx, token); !errors.Is(err, ErrSessionInvalid) {
			t.Fatalf("expired validate = %v, want ErrSessionInvalid", err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&count); err != nil || count != 0 {
			t.Errorf("expired session rows = %d, %v; want 0", count, err)
		}
	})
}

func TestPurgeExpiredSessions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		clock := newFakeClock()
		svc := newTestService(t, db, clock)
		user := setupUser(t, svc)
		if _, _, err := svc.CreateSession(ctx, user.ID); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		clock.Advance(SessionTTL - time.Hour)
		fresh, _, err := svc.CreateSession(ctx, user.ID)
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		clock.Advance(2 * time.Hour)
		removed, err := svc.PurgeExpiredSessions(ctx)
		if err != nil || removed != 1 {
			t.Fatalf("PurgeExpiredSessions = %d, %v; want 1", removed, err)
		}
		if _, err := svc.ValidateSession(ctx, fresh); err != nil {
			t.Errorf("fresh session was purged: %v", err)
		}
	})
}

func TestDeletingUserCascades(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user := setupUser(t, svc)
		if _, _, err := svc.CreateSession(ctx, user.ID); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if _, _, err := svc.CreateAPIKey(ctx, user.ID, "laptop"); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID); err != nil {
			t.Fatalf("delete user: %v", err)
		}
		for _, table := range []string{"sessions", "api_keys"} {
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != 0 {
				t.Errorf("%s rows after user delete = %d, %v; want 0", table, count, err)
			}
		}
	})
}
