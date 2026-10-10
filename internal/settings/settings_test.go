package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/validate"
)

func patch(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad test patch %s: %v", raw, err)
	}
	return out
}

func isValidation(err error) bool {
	var invalid *validate.Error
	return errors.As(err, &invalid)
}

func TestDefaults(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		got, err := svc.Get(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		want := Settings{
			KeepVersions: 10, MaxFilesPerVer: 2500, MaxVersionBytes: 1 << 30, MaxFileBytes: 256 << 20, MaxPendingBytes: 4 << 30,
			BaseDomain: "", SubdomainEnabled: false, PathEnabled: true, CustomDomainsEnabled: true, MaxDomainsPerSite: 10, WildcardStatus: WildcardUnchecked,
			DefaultAccessMode: AccessPublic,
			Email:             Email{Provider: ProviderNone, SMTPPort: 587, SMTPSecurity: SecurityStartTLS},
			TLS:               TLS{DNSProvider: DNSProviderNone},
			Analytics:         Analytics{EnabledByDefault: true, RetentionDays: 395, RawDays: 31, GeoStatus: GeoUnconfigured},
			WhoCanInvite:      InviteRoot, WhoCanCreate: CreateAny,
		}
		if got != want {
			t.Errorf("defaults = %+v, want %+v", got, want)
		}
		if Defaults() != want {
			t.Errorf("Defaults() = %+v, want %+v", Defaults(), want)
		}
		values := Values(got)
		if len(values) != len(definitions) || values[KeyKeepVersions] != int64(10) || values[KeyPathEnabled] != true {
			t.Errorf("Values = %v", values)
		}
	})
}

func TestUpdateStoresAndReturnsValues(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{Now: func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }})
		updated, err := svc.Update(ctx, patch(t, `{"retention.keepVersions": 3, "hosting.baseDomain": "  Sites.Example.COM ", "hosting.pathEnabled": true, "limits.maxFileBytes": 1024}`))
		if err != nil {
			t.Fatal(err)
		}
		if updated.KeepVersions != 3 || updated.BaseDomain != "sites.example.com" || !updated.PathEnabled || updated.MaxFileBytes != 1024 {
			t.Errorf("updated = %+v", updated)
		}
		if updated.MaxFilesPerVer != 2500 || updated.SubdomainEnabled {
			t.Errorf("untouched keys changed: %+v", updated)
		}

		fresh := New(db, Options{})
		reloaded, err := fresh.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded != updated {
			t.Errorf("reloaded = %+v, want %+v", reloaded, updated)
		}

		again, err := svc.Update(ctx, patch(t, `{"retention.keepVersions": 7}`))
		if err != nil {
			t.Fatal(err)
		}
		if again.KeepVersions != 7 || again.BaseDomain != "sites.example.com" {
			t.Errorf("second update = %+v", again)
		}
	})
}

func TestCacheIsInvalidatedOnWrite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		if _, err := svc.Get(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)",
			KeyKeepVersions, "99", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		cached, err := svc.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if cached.KeepVersions != 10 {
			t.Errorf("read bypassed the cache: KeepVersions = %d", cached.KeepVersions)
		}
		if _, err := svc.Update(ctx, patch(t, `{"limits.maxFileBytes": 2048}`)); err != nil {
			t.Fatal(err)
		}
		after, err := svc.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if after.KeepVersions != 99 || after.MaxFileBytes != 2048 {
			t.Errorf("cache survived a write: %+v", after)
		}
	})
}

func TestUpdateRejectsBadInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		bad := map[string]string{
			"unknown key":             `{"retention.keepversions": 3}`,
			"unknown namespace":       `{"nope.nothing": 1}`,
			"zero keep":               `{"retention.keepVersions": 0}`,
			"negative keep":           `{"retention.keepVersions": -1}`,
			"huge keep":               `{"retention.keepVersions": 100000}`,
			"fractional keep":         `{"retention.keepVersions": 1.5}`,
			"string for int":          `{"retention.keepVersions": "3"}`,
			"null":                    `{"retention.keepVersions": null}`,
			"zero max files":          `{"limits.maxFilesPerVersion": 0}`,
			"zero max version bytes":  `{"limits.maxVersionBytes": 0}`,
			"zero max file bytes":     `{"limits.maxFileBytes": 0}`,
			"int for bool":            `{"hosting.pathEnabled": 1}`,
			"string for domains flag": `{"hosting.customDomainsEnabled": "true"}`,
			"zero domain limit":       `{"hosting.maxDomainsPerSite": 0}`,
			"negative domain limit":   `{"hosting.maxDomainsPerSite": -1}`,
			"huge domain limit":       `{"hosting.maxDomainsPerSite": 51}`,
			"fractional domain limit": `{"hosting.maxDomainsPerSite": 2.5}`,
			"string for bool":         `{"hosting.subdomainEnabled": "true"}`,
			"int for string":          `{"hosting.baseDomain": 5}`,
			"domain with scheme":      `{"hosting.baseDomain": "https://example.com"}`,
			"domain with port":        `{"hosting.baseDomain": "example.com:8080"}`,
			"domain with path":        `{"hosting.baseDomain": "example.com/x"}`,
			"domain with wildcard":    `{"hosting.baseDomain": "*.example.com"}`,
			"domain with underscore":  `{"hosting.baseDomain": "my_site.example.com"}`,
			"domain with empty label": `{"hosting.baseDomain": "a..example.com"}`,
			"domain leading hyphen":   `{"hosting.baseDomain": "-a.example.com"}`,
			"long label":              `{"hosting.baseDomain": "` + strings.Repeat("a", 64) + `.example.com"}`,
			"long domain":             `{"hosting.baseDomain": "` + strings.Repeat("a.", 130) + `com"}`,
			"empty patch":             `{}`,
			"read-only status":        `{"hosting.wildcardStatus": "ok"}`,
			"read-only checked at":    `{"hosting.wildcardCheckedAt": "2026-03-01T12:00:00Z"}`,
			"read-only detail":        `{"hosting.wildcardDetail": "fine"}`,
			"password as default":     `{"sites.defaultAccessMode": "password"}`,
			"unknown default mode":    `{"sites.defaultAccessMode": "friends"}`,
			"empty default mode":      `{"sites.defaultAccessMode": ""}`,
			"number for default mode": `{"sites.defaultAccessMode": 1}`,
		}
		for name, raw := range bad {
			_, err := svc.Update(ctx, patch(t, raw))
			if !isValidation(err) {
				t.Errorf("%s: error = %v, want a validation error", name, err)
			}
		}
		got, err := svc.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != Defaults() {
			t.Errorf("a rejected update changed settings: %+v", got)
		}
	})
}

func TestUpdateIsAllOrNothing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		_, err := svc.Update(ctx, patch(t, `{"retention.keepVersions": 4, "limits.maxFileBytes": 0}`))
		if !isValidation(err) {
			t.Fatalf("error = %v, want a validation error", err)
		}
		got, err := svc.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.KeepVersions != 10 {
			t.Errorf("valid half of a rejected patch was applied: %+v", got)
		}
	})
}

func TestBaseDomainCanBeCleared(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		if _, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.com"}`)); err != nil {
			t.Fatal(err)
		}
		cleared, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": ""}`))
		if err != nil {
			t.Fatal(err)
		}
		if cleared.BaseDomain != "" {
			t.Errorf("BaseDomain = %q", cleared.BaseDomain)
		}
	})
}

func TestCorruptStoredValueIsAnError(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		if _, err := db.ExecContext(ctx, "INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)",
			KeyKeepVersions, "many", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err := New(db, Options{}).Get(ctx); err == nil {
			t.Error("a corrupt stored value was ignored")
		}
	})
}

func TestDefaultAccessModeAcceptsPublicAndRestricted(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		got, err := svc.Update(ctx, patch(t, `{"sites.defaultAccessMode": "  Restricted "}`))
		if err != nil {
			t.Fatal(err)
		}
		if got.DefaultAccessMode != AccessRestricted {
			t.Errorf("DefaultAccessMode = %q, want restricted", got.DefaultAccessMode)
		}
		got, err = svc.Update(ctx, patch(t, `{"sites.defaultAccessMode": "public"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got.DefaultAccessMode != AccessPublic {
			t.Errorf("DefaultAccessMode = %q, want public", got.DefaultAccessMode)
		}
	})
}
