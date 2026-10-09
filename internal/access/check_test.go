package access

import (
	"context"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestPublicSitesNeedNoCookie(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		if !e.allowed(e.site.ID) {
			t.Error("a public site refused an anonymous viewer")
		}
		if _, err := e.svc.Check(context.Background(), "missing", nil); err == nil {
			t.Error("Check on an unknown site must fail")
		}
	})
}

func TestPasswordModeAcceptsOnlyPasswordCookies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("password"), Password: setPassword("a shared secret")})
		if e.allowed(e.site.ID) {
			t.Error("anonymous viewer allowed")
		}
		if !e.allowed(e.site.ID, e.cookie(e.site.ID, MethodPassword, "")) {
			t.Error("password cookie refused")
		}
		for _, method := range []Method{MethodEmail, MethodMember} {
			if e.allowed(e.site.ID, e.cookie(e.site.ID, method, e.alex)) {
				t.Errorf("%s cookie accepted in password mode", method)
			}
		}
	})
}

func TestRestrictedModeAcceptsEmailAndMemberCookies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		if e.allowed(e.site.ID) {
			t.Error("anonymous viewer allowed")
		}
		if e.allowed(e.site.ID, e.cookie(e.site.ID, MethodPassword, "")) {
			t.Error("password cookie accepted in restricted mode")
		}
		if !e.allowed(e.site.ID, e.cookie(e.site.ID, MethodEmail, "")) {
			t.Error("email cookie refused")
		}
		if !e.allowed(e.site.ID, e.cookie(e.site.ID, MethodMember, e.alex)) {
			t.Error("member cookie for a workspace admin refused")
		}
		if e.allowed(e.site.ID, e.cookie(e.site.ID, MethodMember, e.sam)) {
			t.Error("member cookie for a user without access accepted")
		}
	})
}

func TestMemberCookieStopsWorkingWhenAccessIsRemoved(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		e.grant("sam@example.com", "read", "site", e.site.ID)
		cookie := e.cookie(e.site.ID, MethodMember, e.sam)
		if !e.allowed(e.site.ID, cookie) {
			t.Fatal("a scoped reader was refused")
		}
		if _, err := db.ExecContext(context.Background(), "DELETE FROM workspace_access WHERE user_id = $1", e.sam); err != nil {
			t.Fatal(err)
		}
		if e.allowed(e.site.ID, cookie) {
			t.Error("the cookie outlived the grant")
		}
	})
}

func TestCookieForOneSiteFailsOnAnother(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		for _, id := range []string{e.site.ID, e.other.ID} {
			e.update(id, ConfigUpdate{Mode: strptr("restricted")})
		}
		cookie := e.cookie(e.site.ID, MethodEmail, "")
		if !e.allowed(e.site.ID, cookie) || e.allowed(e.other.ID, cookie) {
			t.Error("a cookie must open only the site it was issued for")
		}
	})
}

func TestCookieExpiresAfterSevenDays(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		cookie := e.cookie(e.site.ID, MethodEmail, "")
		e.clock.Advance(CookieLifetime - time.Second)
		if !e.allowed(e.site.ID, cookie) {
			t.Error("cookie refused before it expired")
		}
		e.clock.Advance(time.Second)
		if e.allowed(e.site.ID, cookie) {
			t.Error("cookie accepted after it expired")
		}
	})
}

func TestAnyValidCookieInTheListOpensTheSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted")})
		good := e.cookie(e.site.ID, MethodEmail, "")
		if !e.allowed(e.site.ID, "junk", good) {
			t.Error("a valid cookie behind a junk cookie was ignored")
		}
		if e.allowed(e.site.ID, "junk", "v1.x.y") {
			t.Error("junk cookies were accepted")
		}
	})
}

func TestEveryKindOfChangeInvalidatesIssuedCookies(t *testing.T) {
	changes := map[string][]ConfigUpdate{
		"password": {{Password: setPassword("a different secret")}},
		"emails":   {{Emails: listptr("kim@example.com")}},
		"domains":  {{Domains: listptr("example.net")}},
		"mode":     {{Mode: strptr("password")}, {Mode: strptr("restricted")}},
	}
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		for name, steps := range changes {
			site := e.newSite("change-" + name)
			e.update(site.ID, ConfigUpdate{Mode: strptr("restricted"), Password: setPassword("a shared secret")})
			cookie := e.cookie(site.ID, MethodEmail, "")
			if !e.allowed(site.ID, cookie) {
				t.Fatalf("%s: cookie refused before the change", name)
			}
			for _, step := range steps {
				e.update(site.ID, step)
			}
			if e.allowed(site.ID, cookie) {
				t.Errorf("%s: an old cookie still works", name)
			}
		}
	})
}

func TestUserCanReadFollowsGrants(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.grant("sam@example.com", "read", "site", e.site.ID)
		cases := []struct {
			user, site string
			want       bool
		}{
			{e.alex, e.site.ID, true},
			{e.alex, e.other.ID, true},
			{e.sam, e.site.ID, true},
			{e.sam, e.other.ID, false},
			{e.kim, e.site.ID, false},
			{"nobody", e.site.ID, false},
		}
		for _, tc := range cases {
			got, err := e.svc.UserCanRead(ctx, tc.user, tc.site)
			if err != nil || got != tc.want {
				t.Errorf("UserCanRead(%s, %s) = %v, %v; want %v", tc.user, tc.site, got, err, tc.want)
			}
		}
	})
}
