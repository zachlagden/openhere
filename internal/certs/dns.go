package certs

import (
	"context"
	"errors"
	"fmt"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
	"github.com/libdns/libdns"

	"github.com/zachlagden/openhere/internal/settings"
)

var (
	ErrNoDNSProvider = errors.New("no DNS provider is configured for certificates")
	errNoSecretKey   = errors.New("no secret key is loaded")
)

type ProviderBuilder func(token string) certmagic.DNSProvider

type SecretOpener interface {
	Open(sealed string) (string, error)
}

func DefaultProviders() map[string]ProviderBuilder {
	return map[string]ProviderBuilder{
		settings.DNSProviderCloudflare: func(token string) certmagic.DNSProvider {
			return &cloudflare.Provider{APIToken: token}
		},
	}
}

func sealedToken(cfg settings.Settings) (name, sealed string) {
	switch cfg.TLS.DNSProvider {
	case settings.DNSProviderCloudflare:
		return cfg.TLS.DNSProvider, cfg.TLS.CloudflareAPIToken
	default:
		return settings.DNSProviderNone, ""
	}
}

func dnsConfigured(cfg settings.Settings) bool {
	name, sealed := sealedToken(cfg)
	return name != settings.DNSProviderNone && sealed != ""
}

func credentialFingerprint(cfg settings.Settings) string {
	name, sealed := sealedToken(cfg)
	return name + "\x00" + sealed
}

type liveProvider struct {
	settings  SettingsReader
	opener    SecretOpener
	providers map[string]ProviderBuilder
}

func newLiveProvider(reader SettingsReader, opener SecretOpener, providers map[string]ProviderBuilder) *liveProvider {
	if providers == nil {
		providers = DefaultProviders()
	}
	return &liveProvider{settings: reader, opener: opener, providers: providers}
}

func (l *liveProvider) current(ctx context.Context) (certmagic.DNSProvider, error) {
	cfg, err := l.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("read DNS provider settings: %w", err)
	}
	name, sealed := sealedToken(cfg)
	build, known := l.providers[name]
	if !known || sealed == "" {
		return nil, ErrNoDNSProvider
	}
	if l.opener == nil {
		return nil, errNoSecretKey
	}
	token, err := l.opener.Open(sealed)
	if err != nil {
		return nil, errors.New("the stored DNS provider token cannot be opened with the current secret key")
	}
	return build(token), nil
}

func (l *liveProvider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	provider, err := l.current(ctx)
	if err != nil {
		return nil, err
	}
	return provider.AppendRecords(ctx, zone, records)
}

func (l *liveProvider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	provider, err := l.current(ctx)
	if err != nil {
		return nil, err
	}
	return provider.DeleteRecords(ctx, zone, records)
}
