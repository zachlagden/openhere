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

func TestAPIKeyLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user := setupUser(t, svc)

		key, record, err := svc.CreateAPIKey(ctx, user.ID, "  laptop ")
		if err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		if !strings.HasPrefix(key, "sk_") || len(key) != 3+43 {
			t.Errorf("key %q has the wrong shape", key)
		}
		if record.Label != "laptop" || record.Prefix != key[:9] {
			t.Errorf("record = %+v", record)
		}

		var hash string
		if err := db.QueryRowContext(ctx, "SELECT key_hash FROM api_keys WHERE id = $1", record.ID).Scan(&hash); err != nil {
			t.Fatalf("read hash: %v", err)
		}
		if hash == key || hash != HashToken(key) {
			t.Error("the stored key hash is not the SHA-256 of the key")
		}

		got, gotKey, err := svc.AuthenticateAPIKey(ctx, key)
		if err != nil || got.ID != user.ID || gotKey.ID != record.ID {
			t.Fatalf("AuthenticateAPIKey = %+v, %+v, %v", got, gotKey, err)
		}
		if gotKey.LastUsedAt == nil {
			t.Error("last_used_at not recorded")
		}

		if err := svc.RevokeAPIKey(ctx, user.ID, record.ID); err != nil {
			t.Fatalf("RevokeAPIKey: %v", err)
		}
		if err := svc.RevokeAPIKey(ctx, user.ID, record.ID); err != nil {
			t.Errorf("second revoke should be idempotent: %v", err)
		}
		if _, _, err := svc.AuthenticateAPIKey(ctx, key); !errors.Is(err, ErrAPIKeyInvalid) {
			t.Errorf("revoked key error = %v, want ErrAPIKeyInvalid", err)
		}
	})
}

func TestAPIKeyListing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		clock := newFakeClock()
		svc := newTestService(t, db, clock)
		user := setupUser(t, svc)
		other, err := svc.CreateUser(ctx, "sam@example.com", testPassword)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		_, first, err := svc.CreateAPIKey(ctx, user.ID, "first")
		if err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		clock.Advance(time.Minute)
		if _, _, err := svc.CreateAPIKey(ctx, user.ID, "second"); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		if _, _, err := svc.CreateAPIKey(ctx, other.ID, "theirs"); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		keys, err := svc.ListAPIKeys(ctx, user.ID)
		if err != nil || len(keys) != 2 || keys[0].Label != "second" || keys[1].ID != first.ID {
			t.Fatalf("ListAPIKeys = %+v, %v", keys, err)
		}
		if err := svc.RevokeAPIKey(ctx, other.ID, first.ID); !errors.Is(err, ErrAPIKeyNotFound) {
			t.Errorf("revoking another user's key = %v, want ErrAPIKeyNotFound", err)
		}
		if err := svc.RevokeAPIKey(ctx, user.ID, "missing"); !errors.Is(err, ErrAPIKeyNotFound) {
			t.Errorf("revoking a missing key = %v, want ErrAPIKeyNotFound", err)
		}
		empty, err := svc.ListAPIKeys(ctx, "nobody")
		if err != nil || empty == nil || len(empty) != 0 {
			t.Errorf("ListAPIKeys for no keys = %#v, %v; want an empty slice", empty, err)
		}
	})
}

func TestAPIKeyLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user := setupUser(t, svc)
		var first APIKey
		for i := range MaxActiveAPIKeys {
			_, record, err := svc.CreateAPIKey(ctx, user.ID, "key")
			if err != nil {
				t.Fatalf("CreateAPIKey %d: %v", i, err)
			}
			if i == 0 {
				first = record
			}
		}
		if _, _, err := svc.CreateAPIKey(ctx, user.ID, "one too many"); !errors.Is(err, ErrAPIKeyLimit) {
			t.Fatalf("over the limit error = %v, want ErrAPIKeyLimit", err)
		}
		if err := svc.RevokeAPIKey(ctx, user.ID, first.ID); err != nil {
			t.Fatalf("RevokeAPIKey: %v", err)
		}
		if _, _, err := svc.CreateAPIKey(ctx, user.ID, "after revoke"); err != nil {
			t.Errorf("revoking should free a slot: %v", err)
		}
	})
}

func TestAPIKeyRejectsBadInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user := setupUser(t, svc)
		var validation *ValidationError
		if _, _, err := svc.CreateAPIKey(ctx, user.ID, " "); !errors.As(err, &validation) {
			t.Errorf("blank label error = %v, want ValidationError", err)
		}
		if _, _, err := svc.CreateAPIKey(ctx, "missing-user", "label"); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("unknown user error = %v, want ErrUserNotFound", err)
		}
		for _, key := range []string{"", "sk_", "sk_short", "xx_" + strings.Repeat("a", 43), "sk_" + strings.Repeat("a", 43)} {
			if _, _, err := svc.AuthenticateAPIKey(ctx, key); !errors.Is(err, ErrAPIKeyInvalid) {
				t.Errorf("AuthenticateAPIKey(%q) = %v, want ErrAPIKeyInvalid", key, err)
			}
		}
	})
}

func TestAPIKeyLastUsedIsThrottled(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		clock := newFakeClock()
		svc := newTestService(t, db, clock)
		user := setupUser(t, svc)
		key, record, err := svc.CreateAPIKey(ctx, user.ID, "laptop")
		if err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		lastUsed := func() time.Time {
			keys, err := svc.ListAPIKeys(ctx, user.ID)
			if err != nil || len(keys) != 1 || keys[0].ID != record.ID || keys[0].LastUsedAt == nil {
				t.Fatalf("ListAPIKeys = %+v, %v", keys, err)
			}
			return *keys[0].LastUsedAt
		}
		if _, _, err := svc.AuthenticateAPIKey(ctx, key); err != nil {
			t.Fatalf("first use: %v", err)
		}
		first := lastUsed()
		clock.Advance(10 * time.Second)
		if _, _, err := svc.AuthenticateAPIKey(ctx, key); err != nil {
			t.Fatalf("second use: %v", err)
		}
		if !lastUsed().Equal(first) {
			t.Error("last_used_at changed within the throttle interval")
		}
		clock.Advance(2 * time.Minute)
		if _, _, err := svc.AuthenticateAPIKey(ctx, key); err != nil {
			t.Fatalf("third use: %v", err)
		}
		if !lastUsed().After(first) {
			t.Error("last_used_at did not advance after the throttle interval")
		}
	})
}

func TestConcurrentKeyCreationRespectsLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user := setupUser(t, svc)
		for range MaxActiveAPIKeys - 2 {
			if _, _, err := svc.CreateAPIKey(ctx, user.ID, "key"); err != nil {
				t.Fatalf("CreateAPIKey: %v", err)
			}
		}
		results := make(chan error, 6)
		for range 6 {
			go func() {
				_, _, err := svc.CreateAPIKey(ctx, user.ID, "racer")
				results <- err
			}()
		}
		created := 0
		for range 6 {
			err := <-results
			switch {
			case err == nil:
				created++
			case !errors.Is(err, ErrAPIKeyLimit):
				t.Errorf("unexpected error: %v", err)
			}
		}
		if created != 2 {
			t.Errorf("created = %d, want 2", created)
		}
	})
}
