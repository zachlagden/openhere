package access

import (
	"context"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestCheckPassword(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("password"), Password: setPassword("a shared secret")})

		state, ok, err := e.svc.CheckPassword(ctx, e.site.ID, "a shared secret")
		if err != nil || !ok || state.Mode != ModePassword {
			t.Errorf("right password: state = %+v, ok = %v, err = %v", state, ok, err)
		}
		for _, wrong := range []string{"", "a shared secreT", "a shared secret ", "a shared"} {
			if _, ok, err := e.svc.CheckPassword(ctx, e.site.ID, wrong); err != nil || ok {
				t.Errorf("wrong password %q: ok = %v, err = %v", wrong, ok, err)
			}
		}
		if _, _, err := e.svc.CheckPassword(ctx, "missing", "a shared secret"); err == nil {
			t.Error("CheckPassword on an unknown site must fail")
		}
	})
}

func TestCheckPasswordRefusesOutsidePasswordMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted"), Password: setPassword("a shared secret")})
		if _, ok, err := e.svc.CheckPassword(ctx, e.site.ID, "a shared secret"); err != nil || ok {
			t.Errorf("restricted mode accepted a stored password: ok = %v, err = %v", ok, err)
		}
		if _, ok, err := e.svc.CheckPassword(ctx, e.other.ID, "anything at all"); err != nil || ok {
			t.Errorf("public mode accepted a password: ok = %v, err = %v", ok, err)
		}
	})
}

func TestOldPasswordStopsWorkingAfterAChange(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("password"), Password: setPassword("a shared secret")})
		e.update(e.site.ID, ConfigUpdate{Password: setPassword("a new shared secret")})
		if _, ok, _ := e.svc.CheckPassword(ctx, e.site.ID, "a shared secret"); ok {
			t.Error("the old password still works")
		}
		if _, ok, _ := e.svc.CheckPassword(ctx, e.site.ID, "a new shared secret"); !ok {
			t.Error("the new password does not work")
		}
	})
}

func TestStoredPasswordIsAnArgon2idHash(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("password"), Password: setPassword("a shared secret")})
		var hash string
		row := db.QueryRowContext(context.Background(), "SELECT access_password_hash FROM sites WHERE id = $1", e.site.ID)
		if err := row.Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if len(hash) < 20 || !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "a shared secret") {
			t.Errorf("stored hash = %q", hash)
		}
	})
}
