package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestSetupCreatesRootOnce(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())

		needed, err := svc.SetupNeeded(ctx)
		if err != nil || !needed {
			t.Fatalf("SetupNeeded = %v, %v; want true", needed, err)
		}
		root, err := svc.Setup(ctx, "Alex@Example.com", testPassword)
		if err != nil {
			t.Fatalf("Setup: %v", err)
		}
		if !root.IsRoot || root.Email != "alex@example.com" {
			t.Errorf("root = %+v", root)
		}
		needed, err = svc.SetupNeeded(ctx)
		if err != nil || needed {
			t.Fatalf("SetupNeeded after setup = %v, %v; want false", needed, err)
		}
		if _, err := svc.Setup(ctx, "sam@example.com", testPassword); !errors.Is(err, ErrSetupComplete) {
			t.Errorf("second Setup error = %v, want ErrSetupComplete", err)
		}
	})
}

func TestConcurrentSetupHasOneWinner(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := newTestService(t, db, newFakeClock())
		const attempts = 8
		results := make(chan error, attempts)
		var wg sync.WaitGroup
		for i := range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				email := fmt.Sprintf("user%d@example.com", i)
				_, err := svc.Setup(context.Background(), email, testPassword)
				results <- err
			}()
		}
		wg.Wait()
		close(results)

		winners := 0
		for err := range results {
			switch {
			case err == nil:
				winners++
			case !errors.Is(err, ErrSetupComplete):
				t.Errorf("unexpected setup error: %v", err)
			}
		}
		if winners != 1 {
			t.Errorf("winners = %d, want 1", winners)
		}
		var users int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
			t.Fatalf("count users: %v", err)
		}
		if users != 1 {
			t.Errorf("users = %d, want 1", users)
		}
	})
}

func TestSetupRejectsWeakInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := newTestService(t, db, newFakeClock())
		var validation *ValidationError
		if _, err := svc.Setup(context.Background(), "alex@example.com", "short"); !errors.As(err, &validation) {
			t.Errorf("weak password error = %v, want ValidationError", err)
		}
		if _, err := svc.Setup(context.Background(), "not-an-email", testPassword); !errors.As(err, &validation) {
			t.Errorf("bad email error = %v, want ValidationError", err)
		}
		needed, err := svc.SetupNeeded(context.Background())
		if err != nil || !needed {
			t.Errorf("failed setups must leave the instance unconfigured: %v, %v", needed, err)
		}
	})
}

func TestCreateUserRejectsDuplicateEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		user, err := svc.CreateUser(ctx, "sam@example.com", testPassword)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if user.IsRoot {
			t.Error("CreateUser made a root user")
		}
		if _, err := svc.CreateUser(ctx, "SAM@example.com", testPassword); !errors.Is(err, ErrEmailTaken) {
			t.Errorf("duplicate error = %v, want ErrEmailTaken", err)
		}
	})
}

func TestDatabaseEnforcesSingleRootAndLowercaseEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		const insert = "INSERT INTO users (id, email, password_hash, is_root, created_at, updated_at) VALUES ($1, $2, 'x', $3, $4, $4)"
		now := newFakeClock().Now()
		if _, err := db.ExecContext(ctx, insert, "1", "a@example.com", true, now); err != nil {
			t.Fatalf("first root: %v", err)
		}
		if _, err := db.ExecContext(ctx, insert, "2", "b@example.com", true, now); !store.IsUniqueViolation(err) {
			t.Errorf("second root error = %v, want unique violation", err)
		}
		if _, err := db.ExecContext(ctx, insert, "3", "b@example.com", false, now); err != nil {
			t.Errorf("non-root insert: %v", err)
		}
		if _, err := db.ExecContext(ctx, insert, "4", "Upper@example.com", false, now); err == nil {
			t.Error("database accepted a mixed-case email")
		}
	})
}

func TestAuthenticate(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		created, err := svc.CreateUser(ctx, "alex@example.com", testPassword)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		user, err := svc.Authenticate(ctx, " ALEX@example.com", testPassword)
		if err != nil || user.ID != created.ID {
			t.Fatalf("Authenticate = %+v, %v", user, err)
		}
		for name, tc := range map[string][2]string{
			"wrong password": {"alex@example.com", "wrong password here"},
			"unknown email":  {"nobody@example.com", testPassword},
			"malformed":      {"not-an-email", testPassword},
		} {
			if _, err := svc.Authenticate(ctx, tc[0], tc[1]); !errors.Is(err, ErrInvalidCredentials) {
				t.Errorf("%s: error = %v, want ErrInvalidCredentials", name, err)
			}
		}
	})
}

func TestAuthenticateUpgradesOldHashes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		clock := newFakeClock()
		svc := newTestService(t, db, clock)
		if _, err := svc.CreateUser(ctx, "alex@example.com", testPassword); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		stronger := testParams
		stronger.Time = 2
		upgraded := newTestService(t, db, clock)
		upgraded.params = stronger
		if _, err := upgraded.Authenticate(ctx, "alex@example.com", testPassword); err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		var hash string
		if err := db.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE email = $1", "alex@example.com").Scan(&hash); err != nil {
			t.Fatalf("read hash: %v", err)
		}
		if NeedsRehash(hash, stronger) {
			t.Error("stored hash still uses the old parameters")
		}
		if _, err := upgraded.Authenticate(ctx, "alex@example.com", testPassword); err != nil {
			t.Errorf("login after upgrade: %v", err)
		}
	})
}
