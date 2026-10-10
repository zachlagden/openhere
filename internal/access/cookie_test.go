package access

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
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

func TestCookieSignedByAnotherKeyIsRejected(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		first := newEnv(t, db)
		value := cookieFor(t, first, first.site.ID)
		other := New(db, Options{Params: fastParams, Now: first.clock.Now, Secrets: testBox(t, 2)})
		if _, ok, err := other.ParseCookie(context.Background(), value); err != nil || ok {
			t.Errorf("a cookie minted with another box key was accepted (ok = %v, err = %v)", ok, err)
		}
		same := New(db, Options{Params: fastParams, Now: first.clock.Now, Secrets: testBox(t, 1)})
		if _, ok, err := same.ParseCookie(context.Background(), value); err != nil || !ok {
			t.Errorf("a second service with the same key rejected the cookie (ok = %v, err = %v)", ok, err)
		}
	})
}

func TestViewerKeysAreNeverStoredInTheDatabase(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		cookieFor(t, e, e.site.ID)
		if _, err := e.svc.FormToken(context.Background(), e.site.ID, 0, "client"); err != nil {
			t.Fatal(err)
		}
		var count int
		row := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM instance_settings WHERE key LIKE $1", "internal.viewer%")
		if err := row.Scan(&count); err != nil || count != 0 {
			t.Errorf("viewer secret rows = %d, err = %v; want none", count, err)
		}
	})
}

func TestCookieForgedFromADatabaseDumpIsRejected(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		known := bytes.Repeat([]byte{7}, 32)
		_, err := db.ExecContext(context.Background(),
			"INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)",
			"internal.viewerSecret", hex.EncodeToString(known), e.clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		payload := strings.Join([]string{e.site.ID, "0", strconv.FormatInt(e.clock.Now().Add(time.Hour).Unix(), 10), string(MethodPassword), ""}, "|")
		derive := hmac.New(sha256.New, known)
		derive.Write([]byte(cookiePurpose))
		mac := hmac.New(sha256.New, derive.Sum(nil))
		mac.Write([]byte(payload))
		encoding := base64.RawURLEncoding
		forged := cookiePrefix + "." + encoding.EncodeToString([]byte(payload)) + "." + encoding.EncodeToString(mac.Sum(nil))
		if _, ok, err := e.svc.ParseCookie(context.Background(), forged); err != nil || ok {
			t.Errorf("a cookie forged from a database copy was accepted (ok = %v, err = %v)", ok, err)
		}
	})
}
