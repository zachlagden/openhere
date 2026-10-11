package certs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/libdns"

	"github.com/zachlagden/openhere/internal/secrets"
	"github.com/zachlagden/openhere/internal/settings"
)

type recordingProvider struct {
	token    string
	appended []libdns.Record
	zone     string
}

func (r *recordingProvider) AppendRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	r.zone = zone
	r.appended = append(r.appended, records...)
	return records, nil
}

func (r *recordingProvider) DeleteRecords(_ context.Context, _ string, records []libdns.Record) ([]libdns.Record, error) {
	return records, nil
}

func testBox(t *testing.T) *secrets.Box {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{7}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func sealedSettings(t *testing.T, box *secrets.Box, provider, token string) settings.Settings {
	t.Helper()
	cfg := settings.Defaults()
	cfg.TLS.DNSProvider = provider
	if token != "" {
		sealed, err := box.Seal(token)
		if err != nil {
			t.Fatal(err)
		}
		cfg.TLS.CloudflareAPIToken = sealed
	}
	return cfg
}

func TestLiveProviderReadsTheTokenOnEveryCall(t *testing.T) {
	box := testBox(t)
	reader := &fakeSettings{cfg: sealedSettings(t, box, settings.DNSProviderCloudflare, "first-example-token")}
	var built []*recordingProvider
	providers := map[string]ProviderBuilder{
		settings.DNSProviderCloudflare: func(token string) certmagic.DNSProvider {
			p := &recordingProvider{token: token}
			built = append(built, p)
			return p
		},
	}
	live := newLiveProvider(reader, box, providers)
	record := libdns.TXT{Name: "_acme-challenge", Text: "value"}
	if _, err := live.AppendRecords(context.Background(), "example.com.", []libdns.Record{record}); err != nil {
		t.Fatal(err)
	}
	reader.cfg = sealedSettings(t, box, settings.DNSProviderCloudflare, "second-example-token")
	if _, err := live.AppendRecords(context.Background(), "example.com.", []libdns.Record{record}); err != nil {
		t.Fatal(err)
	}
	if len(built) != 2 || built[0].token != "first-example-token" || built[1].token != "second-example-token" {
		t.Errorf("built providers = %+v", built)
	}
	if built[1].zone != "example.com." || len(built[1].appended) != 1 {
		t.Errorf("call was not forwarded: %+v", built[1])
	}
}

func TestLiveProviderFailsWithoutAProviderOrToken(t *testing.T) {
	box := testBox(t)
	for name, cfg := range map[string]settings.Settings{
		"none":          sealedSettings(t, box, settings.DNSProviderNone, ""),
		"missing token": sealedSettings(t, box, settings.DNSProviderCloudflare, ""),
	} {
		live := newLiveProvider(&fakeSettings{cfg: cfg}, box, nil)
		_, err := live.AppendRecords(context.Background(), "example.com.", nil)
		if !errors.Is(err, ErrNoDNSProvider) {
			t.Errorf("%s: error = %v", name, err)
		}
	}
}

func TestLiveProviderErrorsNeverContainTheToken(t *testing.T) {
	box := testBox(t)
	other, err := secrets.New(bytes.Repeat([]byte{9}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	cfg := sealedSettings(t, other, settings.DNSProviderCloudflare, "cf-example-token")
	live := newLiveProvider(&fakeSettings{cfg: cfg}, box, nil)
	_, err = live.DeleteRecords(context.Background(), "example.com.", nil)
	if err == nil || strings.Contains(err.Error(), "cf-example-token") || strings.Contains(err.Error(), cfg.TLS.CloudflareAPIToken) {
		t.Errorf("error = %v", err)
	}
	if _, err := newLiveProvider(&fakeSettings{cfg: cfg}, nil, nil).AppendRecords(context.Background(), "example.com.", nil); !errors.Is(err, errNoSecretKey) {
		t.Errorf("without a key: %v", err)
	}
}

func TestDefaultProvidersIncludeCloudflare(t *testing.T) {
	build, ok := DefaultProviders()[settings.DNSProviderCloudflare]
	if !ok || build("example-token") == nil {
		t.Error("cloudflare is not registered")
	}
}

func TestDNSConfiguredNeedsAProviderAndAToken(t *testing.T) {
	box := testBox(t)
	if dnsConfigured(sealedSettings(t, box, settings.DNSProviderNone, "")) {
		t.Error("none counts as configured")
	}
	if dnsConfigured(sealedSettings(t, box, settings.DNSProviderCloudflare, "")) {
		t.Error("a provider without a token counts as configured")
	}
	if !dnsConfigured(sealedSettings(t, box, settings.DNSProviderCloudflare, "example-token")) {
		t.Error("a provider with a token is not configured")
	}
}
