package settings

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestTLSSettingsDefaultToNoProvider(t *testing.T) {
	defaults := Defaults()
	if defaults.TLS.DNSProvider != DNSProviderNone || defaults.TLS.CloudflareAPIToken != "" {
		t.Errorf("defaults = %+v", defaults.TLS)
	}
	values := Values(defaults)
	if values[KeyTLSDNSProvider] != "none" || values["tls.cloudflareApiTokenSet"] != false {
		t.Errorf("values = %v", values)
	}
}

func TestCloudflareTokenIsSealedAndNeverReported(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{Secrets: testBox(t)})
		updated, err := svc.Update(ctx, patch(t, `{"tls.dnsProvider": "cloudflare", "tls.cloudflareApiToken": "cf-example-token"}`))
		if err != nil {
			t.Fatal(err)
		}
		if updated.TLS.DNSProvider != DNSProviderCloudflare {
			t.Errorf("provider = %q", updated.TLS.DNSProvider)
		}
		sealed := updated.TLS.CloudflareAPIToken
		if sealed == "" || strings.Contains(sealed, "cf-example-token") {
			t.Errorf("the token is not sealed: %q", sealed)
		}
		plain, err := testBox(t).Open(sealed)
		if err != nil || plain != "cf-example-token" {
			t.Errorf("Open = %q, %v", plain, err)
		}
		encoded, err := json.Marshal(Values(updated))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "cf-example-token") || strings.Contains(string(encoded), sealed) {
			t.Fatalf("the token leaked into the values: %s", encoded)
		}
		if Values(updated)["tls.cloudflareApiTokenSet"] != true {
			t.Errorf("the set flag is missing: %s", encoded)
		}
	})
}

func TestCloudflareTokenCanBeReplacedAndCleared(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{Secrets: testBox(t)})
		if _, err := svc.Update(ctx, patch(t, `{"tls.cloudflareApiToken": "first-example"}`)); err != nil {
			t.Fatal(err)
		}
		replaced, err := svc.Update(ctx, patch(t, `{"tls.cloudflareApiToken": "second-example"}`))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := testBox(t).Open(replaced.TLS.CloudflareAPIToken)
		if err != nil || plain != "second-example" {
			t.Errorf("Open = %q, %v", plain, err)
		}
		cleared, err := svc.Update(ctx, patch(t, `{"tls.cloudflareApiToken": ""}`))
		if err != nil {
			t.Fatal(err)
		}
		if cleared.TLS.CloudflareAPIToken != "" {
			t.Error("the token survived clearing")
		}
	})
}

func TestTLSSettingsRejectBadInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{Secrets: testBox(t)})
		for name, body := range map[string]string{
			"provider":    `{"tls.dnsProvider": "route53"}`,
			"token type":  `{"tls.cloudflareApiToken": 5}`,
			"token null":  `{"tls.cloudflareApiToken": null}`,
			"token lines": `{"tls.cloudflareApiToken": "a\nb"}`,
			"set flag":    `{"tls.cloudflareApiTokenSet": true}`,
		} {
			if _, err := svc.Update(context.Background(), patch(t, body)); !isValidation(err) {
				t.Errorf("%s: error = %v, want a validation error", name, err)
			}
		}
	})
}
