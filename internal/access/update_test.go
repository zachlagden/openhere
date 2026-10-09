package access

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestNewSitesAreOpenWithAnEmptyAllowlist(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		cfg, err := e.svc.Get(context.Background(), e.site.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Mode != ModePublic || cfg.HasPassword || len(cfg.Emails) != 0 || len(cfg.Domains) != 0 {
			t.Errorf("config = %+v", cfg)
		}
		if _, err := e.svc.Get(context.Background(), "missing"); err == nil {
			t.Error("Get on an unknown site must fail")
		}
	})
}

func TestUpdateStoresModePasswordAndLists(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		cfg := e.update(e.site.ID, ConfigUpdate{
			Mode: strptr("password"), Password: setPassword("a shared secret"),
			Emails: listptr("Sam@Example.com", "alex@example.com", "sam@example.com"), Domains: listptr("@Example.org", "example.net"),
		})
		if cfg.Mode != ModePassword || !cfg.HasPassword {
			t.Errorf("config = %+v", cfg)
		}
		if !slices.Equal(cfg.Emails, []string{"alex@example.com", "sam@example.com"}) {
			t.Errorf("emails = %v", cfg.Emails)
		}
		if !slices.Equal(cfg.Domains, []string{"example.net", "example.org"}) {
			t.Errorf("domains = %v", cfg.Domains)
		}
		reread, err := e.svc.Get(context.Background(), e.site.ID)
		if err != nil || reread.Mode != ModePassword || len(reread.Emails) != 2 {
			t.Errorf("reread = %+v, err = %v", reread, err)
		}
		untouched, err := e.svc.Get(context.Background(), e.other.ID)
		if err != nil || untouched.Mode != ModePublic || len(untouched.Emails) != 0 {
			t.Errorf("another site changed: %+v, err = %v", untouched, err)
		}
	})
}

func TestPasswordModeNeedsAPassword(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		if _, err := e.svc.Update(ctx, e.site.ID, ConfigUpdate{Mode: strptr("password")}); !isValidation(err) {
			t.Fatalf("password mode without a password: %v", err)
		}
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("password"), Password: setPassword("a shared secret")})
		if _, err := e.svc.Update(ctx, e.site.ID, ConfigUpdate{Password: clearPassword()}); !isValidation(err) {
			t.Fatalf("clearing the password while in password mode: %v", err)
		}
		cfg := e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted"), Password: clearPassword()})
		if cfg.Mode != ModeRestricted || cfg.HasPassword {
			t.Errorf("config = %+v", cfg)
		}
		if _, err := e.svc.Update(ctx, e.site.ID, ConfigUpdate{Mode: strptr("password")}); !isValidation(err) {
			t.Fatalf("password mode after clearing: %v", err)
		}
	})
}

func TestFailedUpdatesChangeNothing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.update(e.site.ID, ConfigUpdate{Emails: listptr("sam@example.com")})
		before := e.epoch(e.site.ID)
		_, err := e.svc.Update(ctx, e.site.ID, ConfigUpdate{Emails: listptr("kim@example.com"), Mode: strptr("password")})
		if !isValidation(err) {
			t.Fatalf("error = %v", err)
		}
		cfg, err := e.svc.Get(ctx, e.site.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(cfg.Emails, []string{"sam@example.com"}) || cfg.Mode != ModePublic || e.epoch(e.site.ID) != before {
			t.Errorf("a failed update leaked: %+v epoch %d", cfg, e.epoch(e.site.ID))
		}
	})
}

func TestUpdateValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		many := make([]string, MaxAllowlistEntries+1)
		for i := range many {
			many[i] = "user" + strings.Repeat("a", i%5) + string(rune('a'+i%26)) + "@example.com"
		}
		manyDomains := make([]string, MaxAllowlistEntries+1)
		for i := range manyDomains {
			manyDomains[i] = "d" + strings.Repeat("x", i%7) + ".example.com"
		}
		bad := map[string]ConfigUpdate{
			"unknown mode":        {Mode: strptr("friends")},
			"empty mode":          {Mode: strptr("")},
			"upper-case mode":     {Mode: strptr("Public")},
			"short password":      {Password: setPassword("short")},
			"long password":       {Password: setPassword(strings.Repeat("p", MaxPasswordLength+1))},
			"bad email":           {Emails: listptr("not-an-email")},
			"non-ascii email":     {Emails: listptr("zoë@example.com")},
			"display name email":  {Emails: listptr("Alex <alex@example.com>")},
			"empty email":         {Emails: listptr("")},
			"too many emails":     {Emails: &many},
			"domain with scheme":  {Domains: listptr("https://example.com")},
			"domain with path":    {Domains: listptr("example.com/x")},
			"domain wildcard":     {Domains: listptr("*.example.com")},
			"domain without dot":  {Domains: listptr("com")},
			"domain empty label":  {Domains: listptr("a..example.com")},
			"empty domain":        {Domains: listptr(" ")},
			"domain with space":   {Domains: listptr("exa mple.com")},
			"domain with at sign": {Domains: listptr("alex@example.com")},
			"too many domains":    {Domains: &manyDomains},
		}
		for name, in := range bad {
			if _, err := e.svc.Update(context.Background(), e.site.ID, in); !isValidation(err) {
				t.Errorf("%s: error = %v, want a validation error", name, err)
			}
		}
		exact := make([]string, MaxAllowlistEntries)
		for i := range exact {
			exact[i] = "user" + strings.Repeat("a", i/26) + string(rune('a'+i%26)) + "@example.com"
		}
		if _, err := e.svc.Update(context.Background(), e.site.ID, ConfigUpdate{Emails: &exact}); err != nil {
			t.Errorf("exactly %d entries must be accepted: %v", MaxAllowlistEntries, err)
		}
	})
}

func TestEpochMovesOnEveryKindOfChangeAndOnlyThen(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		id := e.site.ID
		steps := []struct {
			name   string
			update ConfigUpdate
			moves  bool
		}{
			{"mode to restricted", ConfigUpdate{Mode: strptr("restricted")}, true},
			{"same mode", ConfigUpdate{Mode: strptr("restricted")}, false},
			{"add an email", ConfigUpdate{Emails: listptr("sam@example.com")}, true},
			{"same emails", ConfigUpdate{Emails: listptr("sam@example.com")}, false},
			{"same emails in another case", ConfigUpdate{Emails: listptr("SAM@example.com")}, false},
			{"add a domain", ConfigUpdate{Domains: listptr("example.org")}, true},
			{"same domains", ConfigUpdate{Domains: listptr("example.org")}, false},
			{"clear emails", ConfigUpdate{Emails: listptr()}, true},
			{"clear domains", ConfigUpdate{Domains: listptr()}, true},
			{"set a password", ConfigUpdate{Password: setPassword("a shared secret")}, true},
			{"set a password again", ConfigUpdate{Password: setPassword("a shared secret")}, true},
			{"mode to password", ConfigUpdate{Mode: strptr("password")}, true},
			{"change the password", ConfigUpdate{Password: setPassword("another secret")}, true},
			{"mode to public", ConfigUpdate{Mode: strptr("public")}, true},
			{"clear the password", ConfigUpdate{Password: clearPassword()}, true},
			{"clear a missing password", ConfigUpdate{Password: clearPassword()}, false},
			{"empty update", ConfigUpdate{}, false},
		}
		for _, step := range steps {
			before := e.epoch(id)
			e.update(id, step.update)
			moved := e.epoch(id) != before
			if moved != step.moves {
				t.Errorf("%s: epoch moved = %v, want %v", step.name, moved, step.moves)
			}
		}
		if e.epoch(e.other.ID) != 0 {
			t.Error("another site's epoch moved")
		}
	})
}

func TestUpdateDiscardsOutstandingCodesAndHandoffs(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		e.update(e.site.ID, ConfigUpdate{Mode: strptr("restricted"), Emails: listptr("sam@example.com")})
		if err := e.svc.RequestCode(ctx, CodeRequest{SiteID: e.site.ID, SiteLabel: "handbook", Email: "sam@example.com"}); err != nil {
			t.Fatal(err)
		}
		token, err := e.svc.IssueHandoff(ctx, e.site.ID, e.alex)
		if err != nil {
			t.Fatal(err)
		}
		e.update(e.site.ID, ConfigUpdate{Domains: listptr("example.org")})

		_, ok, err := e.svc.VerifyCode(ctx, e.site.ID, "sam@example.com", lastCode(t, e))
		if err != nil || ok {
			t.Errorf("a code survived the change: ok = %v, err = %v", ok, err)
		}
		if _, _, ok, err := e.svc.ConsumeHandoff(ctx, e.site.ID, token); err != nil || ok {
			t.Errorf("a handoff survived the change: ok = %v, err = %v", ok, err)
		}
	})
}
