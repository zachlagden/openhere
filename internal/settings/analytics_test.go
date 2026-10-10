package settings

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestAnalyticsDefaults(t *testing.T) {
	defaults := Defaults().Analytics
	if !defaults.EnabledByDefault || defaults.RetentionDays != 395 || defaults.RawDays != 31 || defaults.GeoStatus != GeoUnconfigured {
		t.Errorf("defaults = %+v", defaults)
	}
	values := Values(Defaults())
	if values["analytics.maxmindLicenseKeySet"] != false || values[KeyGeoUpdatedAt] != nil {
		t.Errorf("values = %v", values)
	}
}

func TestAnalyticsRetentionValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		ctx := context.Background()
		for _, bad := range []string{"-1", "1", "29", "3651", `"x"`} {
			if _, err := svc.Update(ctx, patch(t, `{"analytics.retentionDays": `+bad+`}`)); !isValidation(err) {
				t.Errorf("retentionDays %s: err = %v, want validation error", bad, err)
			}
		}
		for _, bad := range []string{"0", "13", "366"} {
			if _, err := svc.Update(ctx, patch(t, `{"analytics.rawDays": `+bad+`}`)); !isValidation(err) {
				t.Errorf("rawDays %s: err = %v, want validation error", bad, err)
			}
		}
		updated, err := svc.Update(ctx, patch(t, `{"analytics.retentionDays": 0, "analytics.rawDays": 14, "analytics.enabledByDefault": false}`))
		if err != nil {
			t.Fatal(err)
		}
		if updated.Analytics.RetentionDays != 0 || updated.Analytics.RawDays != 14 || updated.Analytics.EnabledByDefault {
			t.Errorf("updated = %+v", updated.Analytics)
		}
	})
}

func TestMaxMindLicenseKeyIsSealedAndNeverReported(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{Secrets: testBox(t)})
		updated, err := svc.Update(ctx, patch(t, `{"analytics.maxmindAccountId": " 123456 ", "analytics.maxmindLicenseKey": "licence-example-key"}`))
		if err != nil {
			t.Fatal(err)
		}
		if updated.Analytics.MaxMindAccountID != "123456" {
			t.Errorf("account id = %q", updated.Analytics.MaxMindAccountID)
		}
		sealed := updated.Analytics.MaxMindLicenseKey
		if sealed == "" || strings.Contains(sealed, "licence-example-key") {
			t.Errorf("the key is not sealed: %q", sealed)
		}
		encoded, err := json.Marshal(Values(updated))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "licence-example-key") || strings.Contains(string(encoded), sealed) {
			t.Fatalf("the key leaked into the values: %s", encoded)
		}
		if Values(updated)["analytics.maxmindLicenseKeySet"] != true {
			t.Errorf("the set flag is missing: %s", encoded)
		}
		if _, err := svc.Update(ctx, patch(t, `{"analytics.maxmindAccountId": "abc"}`)); !isValidation(err) {
			t.Errorf("non numeric account id: err = %v", err)
		}
	})
}

func TestGeoStateIsReadOnlyForClientsAndRecordedByTheServer(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		for _, key := range []string{KeyGeoStatus, KeyGeoUpdatedAt, KeyGeoCheckedAt, KeyGeoDetail} {
			if _, err := svc.Update(ctx, patch(t, `{"`+key+`": "x"}`)); !isValidation(err) {
				t.Errorf("%s: err = %v, want read-only error", key, err)
			}
		}
		if err := svc.RecordGeo(ctx, GeoState{Status: GeoOK, Detail: "fine", Updated: true}); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.Analytics.GeoStatus != GeoOK || got.Analytics.GeoDetail != "fine" || got.Analytics.GeoUpdatedAt == "" || got.Analytics.GeoCheckedAt == "" {
			t.Errorf("geo state = %+v", got.Analytics)
		}
	})
}
