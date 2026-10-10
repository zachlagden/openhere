package certs

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/ratelimit"
	"github.com/zachlagden/selfhost/internal/settings"
)

var ErrRefused = errors.New("a certificate is not available for this name")

var (
	errNotVerified   = errors.New("not a verified custom domain")
	errFeatureOff    = errors.New("custom domains are switched off")
	errSiteSubdomain = errors.New("site subdomains use the wildcard certificate")
	errRateLimited   = errors.New("certificate issuance is rate limited")
	errUnavailable   = errors.New("the decision could not be made")
)

type SettingsReader interface {
	Get(ctx context.Context) (settings.Settings, error)
}

type DomainLookup interface {
	Lookup(ctx context.Context, host string) (domains.Target, bool, error)
}

type Limits struct {
	Total   ratelimit.Rule
	PerName ratelimit.Rule
	PerSite ratelimit.Rule
}

func DefaultLimits() Limits {
	return Limits{
		Total:   ratelimit.Rule{Max: 50, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 5, Window: time.Hour},
		PerSite: ratelimit.Rule{Max: 10, Window: time.Hour},
	}
}

type PolicyOptions struct {
	ManagementHost string
	Limits         Limits
	Now            func() time.Time
	Logger         *slog.Logger
}

type Policy struct {
	settings SettingsReader
	domains  DomainLookup
	host     string
	total    *ratelimit.Limiter
	perName  *ratelimit.Limiter
	perSite  *ratelimit.Limiter
	reserve  sync.Mutex
	refusals *refusalCounter
}

func NewPolicy(settingsReader SettingsReader, lookup DomainLookup, opts PolicyOptions) *Policy {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	limits := opts.Limits
	if limits == (Limits{}) {
		limits = DefaultLimits()
	}
	return &Policy{
		settings: settingsReader,
		domains:  lookup,
		host:     normaliseName(opts.ManagementHost),
		total:    ratelimit.New(limits.Total.Max, limits.Total.Window, now),
		perName:  ratelimit.New(limits.PerName.Max, limits.PerName.Window, now),
		perSite:  ratelimit.New(limits.PerSite.Max, limits.PerSite.Window, now),
		refusals: newRefusalCounter(logger, now),
	}
}

func (p *Policy) Allow(ctx context.Context, name string) error {
	if err := p.decide(ctx, normaliseName(name)); err != nil {
		p.refusals.note(err.Error())
		return ErrRefused
	}
	return nil
}

func (p *Policy) decide(ctx context.Context, name string) error {
	if name == p.host {
		return nil
	}
	cfg, err := p.settings.Get(ctx)
	if err != nil {
		return errUnavailable
	}
	if underBase(name, cfg.BaseDomain) {
		return errSiteSubdomain
	}
	if !cfg.CustomDomainsEnabled {
		return errFeatureOff
	}
	_, serves, err := p.domains.Lookup(ctx, name)
	if err != nil {
		return errUnavailable
	}
	if !serves {
		return errNotVerified
	}
	return nil
}

func normaliseName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
}

func underBase(name, base string) bool {
	return base != "" && (name == base || strings.HasSuffix(name, "."+base))
}
