package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

type adminFixture struct {
	svc  *Service
	root User
	sam  User
}

func newAdminFixture(t *testing.T, db store.Store) adminFixture {
	t.Helper()
	ctx := context.Background()
	svc := newTestService(t, db, newFakeClock())
	root, err := svc.Setup(ctx, "alex@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	sam, err := svc.CreateUser(ctx, "sam@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	return adminFixture{svc: svc, root: root, sam: sam}
}

func TestDisabledUserCannotLogIn(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		f := newAdminFixture(t, db)
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Authenticate(ctx, "sam@example.com", testPassword); !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("Authenticate error = %v, want ErrUserDisabled", err)
		}
		if _, err := f.svc.Authenticate(ctx, "sam@example.com", "wrong password here"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("a wrong password for a disabled user must look like any other wrong password, got %v", err)
		}
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, f.sam.ID, false); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Authenticate(ctx, "sam@example.com", testPassword); err != nil {
			t.Fatalf("an enabled user cannot log in: %v", err)
		}
	})
}

func TestDisablingEndsSessionsAndStopsKeys(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		f := newAdminFixture(t, db)
		token, _, err := f.svc.CreateSession(ctx, f.sam.ID)
		if err != nil {
			t.Fatal(err)
		}
		key, _, err := f.svc.CreateAPIKey(ctx, f.sam.ID, "laptop")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.ValidateSession(ctx, token); !errors.Is(err, ErrSessionInvalid) {
			t.Errorf("ValidateSession error = %v, want ErrSessionInvalid", err)
		}
		if _, _, err := f.svc.AuthenticateAPIKey(ctx, key); !errors.Is(err, ErrAPIKeyInvalid) {
			t.Errorf("AuthenticateAPIKey error = %v, want ErrAPIKeyInvalid", err)
		}
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, f.sam.ID, false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.svc.AuthenticateAPIKey(ctx, key); err != nil {
			t.Errorf("an enabled user's key does not work: %v", err)
		}
		if _, err := f.svc.ValidateSession(ctx, token); !errors.Is(err, ErrSessionInvalid) {
			t.Error("a session that was ended came back after enabling")
		}
	})
}

func TestDisableGuards(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		f := newAdminFixture(t, db)
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, f.root.ID, true); !errors.Is(err, ErrSelfDisable) {
			t.Errorf("self disable error = %v, want ErrSelfDisable", err)
		}
		if _, err := f.svc.SetDisabled(ctx, f.sam.ID, f.root.ID, true); !errors.Is(err, ErrLastRoot) {
			t.Errorf("last root error = %v, want ErrLastRoot", err)
		}
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, "no-such-user", true); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("unknown user error = %v, want ErrUserNotFound", err)
		}
		got, err := f.svc.UserByID(ctx, f.root.ID)
		if err != nil || got.Disabled() {
			t.Errorf("the root user was disabled: %+v, %v", got, err)
		}
	})
}

func TestListUsersAndLookupByEmail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		f := newAdminFixture(t, db)
		if _, err := f.svc.SetDisabled(ctx, f.root.ID, f.sam.ID, true); err != nil {
			t.Fatal(err)
		}
		users, err := f.svc.ListUsers(ctx)
		if err != nil || len(users) != 2 || users[0].Email != "alex@example.com" || users[1].Email != "sam@example.com" {
			t.Fatalf("users = %+v, %v", users, err)
		}
		if users[0].Disabled() || !users[1].Disabled() {
			t.Errorf("disabled flags = %v, %v", users[0].Disabled(), users[1].Disabled())
		}
		found, err := f.svc.UserByEmail(ctx, " SAM@example.com ")
		if err != nil || found.ID != f.sam.ID {
			t.Errorf("UserByEmail = %+v, %v", found, err)
		}
		if _, err := f.svc.UserByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("unknown email error = %v", err)
		}
		if _, err := f.svc.UserByEmail(ctx, "not an email"); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("malformed email error = %v", err)
		}
	})
}

func TestInsertMemberStoresTheDisplayName(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := newTestService(t, db, newFakeClock())
		creds, err := svc.PrepareCredentials("kim@example.com", testPassword)
		if err != nil {
			t.Fatal(err)
		}
		var created User
		err = store.WithTx(ctx, db, func(q store.Querier) error {
			var insertErr error
			created, insertErr = svc.InsertMember(ctx, q, creds, "Kim Example")
			return insertErr
		})
		if err != nil {
			t.Fatal(err)
		}
		got, err := svc.UserByID(ctx, created.ID)
		if err != nil || got.DisplayName != "Kim Example" || got.IsRoot {
			t.Fatalf("user = %+v, %v", got, err)
		}
	})
}
