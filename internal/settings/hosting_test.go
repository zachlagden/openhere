package settings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestBaseDomainFallsBackToTheConfiguredDefault(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{DefaultBaseDomain: "Sites.Example.com"})
		got, err := svc.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.BaseDomain != "sites.example.com" {
			t.Errorf("default base domain = %q", got.BaseDomain)
		}
		set, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.org"}`))
		if err != nil || set.BaseDomain != "example.org" {
			t.Fatalf("explicit domain = %q, %v", set.BaseDomain, err)
		}
		cleared, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": ""}`))
		if err != nil || cleared.BaseDomain != "sites.example.com" {
			t.Errorf("cleared domain = %q, %v, want the default back", cleared.BaseDomain, err)
		}
	})
}

func TestUnusableDefaultBaseDomainIsIgnored(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{DefaultBaseDomain: "2001:db8::1"})
		got, err := svc.Get(context.Background())
		if err != nil || got.BaseDomain != "" {
			t.Errorf("base domain = %q, %v, want empty", got.BaseDomain, err)
		}
	})
}

func TestSubdomainModeNeedsAVerifiedWildcard(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{Now: func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }})
		enable := patch(t, `{"hosting.subdomainEnabled": true}`)

		if _, err := svc.Update(ctx, enable); !errors.Is(err, ErrWildcardNotVerified) {
			t.Fatalf("unchecked: error = %v, want ErrWildcardNotVerified", err)
		}
		if _, err := svc.RecordWildcard(ctx, false, "no record"); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Update(ctx, enable); !errors.Is(err, ErrWildcardNotVerified) {
			t.Fatalf("failed: error = %v, want ErrWildcardNotVerified", err)
		}
		verified, err := svc.RecordWildcard(ctx, true, "")
		if err != nil {
			t.Fatal(err)
		}
		if verified.WildcardStatus != WildcardOK || verified.WildcardCheckedAt != "2026-03-01T12:00:00Z" {
			t.Errorf("verified = %+v", verified)
		}
		on, err := svc.Update(ctx, enable)
		if err != nil || !on.SubdomainEnabled {
			t.Fatalf("enable after ok: %+v, %v", on, err)
		}
	})
}

func TestChangingTheBaseDomainResetsTheWildcardCheck(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		if _, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.com"}`)); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.RecordWildcard(ctx, true, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Update(ctx, patch(t, `{"hosting.subdomainEnabled": true}`)); err != nil {
			t.Fatal(err)
		}

		same, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.com"}`))
		if err != nil || !same.SubdomainEnabled || same.WildcardStatus != WildcardOK {
			t.Fatalf("rewriting the same domain changed the check: %+v, %v", same, err)
		}
		moved, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.org"}`))
		if err != nil {
			t.Fatal(err)
		}
		if moved.SubdomainEnabled || moved.WildcardStatus != WildcardUnchecked || moved.WildcardCheckedAt != "" {
			t.Errorf("after moving the domain = %+v", moved)
		}
		if _, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.net", "hosting.subdomainEnabled": true}`)); !errors.Is(err, ErrWildcardNotVerified) {
			t.Errorf("enabling together with a domain change: error = %v", err)
		}
	})
}

func TestWildcardValuesInTheAPIMap(t *testing.T) {
	values := Values(Defaults())
	if values[KeyWildcardStatus] != WildcardUnchecked || values[KeyWildcardChecked] != nil || values[KeyWildcardDetail] != "" {
		t.Errorf("wildcard defaults = %v", values)
	}
}
