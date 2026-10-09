package access

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestCookieRoundTrip(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		value, expires, err := e.svc.IssueCookie(ctx, e.site.ID, 3, MethodMember, e.sam)
		if err != nil {
			t.Fatal(err)
		}
		if want := e.clock.Now().Add(CookieLifetime); !expires.Equal(want) {
			t.Errorf("expires = %v, want %v", expires, want)
		}
		claims, ok, err := e.svc.ParseCookie(ctx, value)
		if err != nil || !ok {
			t.Fatalf("ParseCookie: ok = %v, err = %v", ok, err)
		}
		if claims.SiteID != e.site.ID || claims.Epoch != 3 || claims.Method != MethodMember || claims.Subject != e.sam || !claims.Expires.Equal(expires) {
			t.Errorf("claims = %+v", claims)
		}
		if strings.ContainsAny(value, " ;,\"\\") {
			t.Errorf("cookie value %q has characters that need quoting", value)
		}
	})
}

func TestCookieLifetimeIsSevenDays(t *testing.T) {
	if CookieLifetime != 7*24*time.Hour {
		t.Errorf("CookieLifetime = %v", CookieLifetime)
	}
}

func TestTamperedCookiesAreRejected(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		value, _, err := e.svc.IssueCookie(ctx, e.site.ID, 0, MethodPassword, "")
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(value, ".")
		flip := func(s string) string {
			if s[0] == 'A' {
				return "B" + s[1:]
			}
			return "A" + s[1:]
		}
		variants := map[string]string{
			"payload changed":     parts[0] + "." + flip(parts[1]) + "." + parts[2],
			"mac changed":         parts[0] + "." + parts[1] + "." + flip(parts[2]),
			"mac dropped":         parts[0] + "." + parts[1] + ".",
			"version changed":     "v2." + parts[1] + "." + parts[2],
			"extra part":          value + ".x",
			"empty":               "",
			"garbage":             "not a cookie",
			"payload from mac":    parts[0] + "." + parts[2] + "." + parts[2],
			"oversized":           strings.Repeat("a", maxCookieLength+1),
			"not base64":          "v1.!!!.???",
			"truncated mac":       parts[0] + "." + parts[1] + "." + parts[2][:10],
			"mac of another site": cookieFor(t, e, e.other.ID)[:strings.LastIndex(cookieFor(t, e, e.other.ID), ".")] + "." + parts[2],
		}
		for name, tampered := range variants {
			if _, ok, err := e.svc.ParseCookie(ctx, tampered); err != nil || ok {
				t.Errorf("%s: accepted (ok = %v, err = %v)", name, ok, err)
			}
		}
	})
}

func cookieFor(t *testing.T, e *env, siteID string) string {
	t.Helper()
	value, _, err := e.svc.IssueCookie(context.Background(), siteID, 0, MethodPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestCookieSignedByAnotherInstanceIsRejected(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		first := newEnv(t, db)
		value := cookieFor(t, first, first.site.ID)
		secondDB := newEnvOnFreshSecret(t, db)
		if _, ok, err := secondDB.svc.ParseCookie(context.Background(), value); err != nil || ok {
			t.Errorf("a cookie signed with another secret was accepted (ok = %v, err = %v)", ok, err)
		}
	})
}

func newEnvOnFreshSecret(t *testing.T, db store.Store) *env {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), "DELETE FROM instance_settings WHERE key = $1", secretSettingKey); err != nil {
		t.Fatal(err)
	}
	fresh := New(db, Options{Params: fastParams})
	return &env{t: t, svc: fresh}
}

func TestViewerSecretIsGeneratedOnceAndShared(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		value := cookieFor(t, e, e.site.ID)
		peer := New(db, Options{Params: fastParams, Now: e.clock.Now})
		if _, ok, err := peer.ParseCookie(context.Background(), value); err != nil || !ok {
			t.Fatalf("a second service on the same database rejected the cookie (ok = %v, err = %v)", ok, err)
		}
		var count int
		row := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM instance_settings WHERE key = $1", secretSettingKey)
		if err := row.Scan(&count); err != nil || count != 1 {
			t.Errorf("secret rows = %d, err = %v", count, err)
		}
		var stored string
		row = db.QueryRowContext(context.Background(), "SELECT value FROM instance_settings WHERE key = $1", secretSettingKey)
		if err := row.Scan(&stored); err != nil || len(stored) != 2*secretBytes {
			t.Errorf("stored secret has length %d, err = %v", len(stored), err)
		}
	})
}
