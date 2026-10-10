package hosting

import (
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/settings"
)

const (
	StateOn  = "on"
	StateOff = "off"

	ReasonSwitchedOff    = "switched_off"
	ReasonWildcardFailed = "wildcard_check_failed"
	ReasonHTTPSRequired  = "https_required"
	ReasonNotApplicable  = "not_applicable"
)

type ModeStatus struct {
	State  string
	Reason string
}

type Notice struct {
	Code    string
	Message string
}

type SiteFacts struct {
	Slug    string
	Live    bool
	Gated   bool
	Domains domains.Summary
}

type SiteAvailability struct {
	Reachable     bool
	URL           string
	Subdomain     ModeStatus
	Path          ModeStatus
	CustomDomains ModeStatus
	Notices       []Notice
}

type InstanceAvailability struct {
	Subdomain     ModeStatus
	Path          ModeStatus
	CustomDomains ModeStatus
	Notices       []Notice
	Degraded      bool
}

func subdomainStatus(cfg settings.Settings) ModeStatus {
	switch {
	case !cfg.SubdomainEnabled && cfg.SubdomainDisabledReason == settings.DisabledByWildcard:
		return ModeStatus{StateOff, ReasonWildcardFailed}
	case !cfg.SubdomainEnabled:
		return ModeStatus{StateOff, ReasonSwitchedOff}
	case cfg.BaseDomain == "":
		return ModeStatus{StateOff, ReasonNotApplicable}
	case cfg.WildcardStatus == settings.WildcardFailed:
		return ModeStatus{StateOn, ReasonWildcardFailed}
	default:
		return ModeStatus{State: StateOn}
	}
}

func pathStatus(cfg settings.Settings) ModeStatus {
	if !cfg.PathEnabled {
		return ModeStatus{StateOff, ReasonSwitchedOff}
	}
	return ModeStatus{State: StateOn}
}

func customDomainsStatus(cfg settings.Settings) ModeStatus {
	if !cfg.CustomDomainsEnabled {
		return ModeStatus{StateOff, ReasonSwitchedOff}
	}
	return ModeStatus{State: StateOn}
}

func InstanceHosting(cfg settings.Settings) InstanceAvailability {
	subdomain, path := subdomainStatus(cfg), pathStatus(cfg)
	var notices []Notice
	switch {
	case subdomain.State == StateOff:
		notices = append(notices, Notice{"subdomain_off", "Subdomain addresses are not available for any site right now."})
	case subdomain.Reason == ReasonWildcardFailed:
		notices = append(notices, Notice{"subdomain_unreliable", "Subdomain addresses might not work right now."})
	}
	if path.State == StateOff {
		notices = append(notices, Notice{"path_off", "Path addresses are switched off for all sites."})
	}
	custom := customDomainsStatus(cfg)
	if custom.State == StateOff {
		notices = append(notices, Notice{"custom_domains_off", "Custom domains are switched off for all sites."})
	}
	return InstanceAvailability{
		Subdomain: subdomain, Path: path, CustomDomains: custom, Notices: notices,
		Degraded: subdomain.Reason == ReasonWildcardFailed,
	}
}

func SiteHosting(public config.PublicURL, cfg settings.Settings, facts SiteFacts) SiteAvailability {
	subdomain, path := subdomainStatus(cfg), pathStatus(cfg)
	if path.State == StateOn && facts.Gated && !public.Secure() {
		path = ModeStatus{StateOff, ReasonHTTPSRequired}
	}
	custom := customDomainsStatus(cfg)
	customServes := custom.State == StateOn && facts.Domains.Verified > 0
	reachable := facts.Live && (subdomain.State == StateOn || path.State == StateOn || customServes)
	return SiteAvailability{
		Reachable:     reachable,
		URL:           PreferredURL(public, cfg, facts.Slug, facts.Domains),
		Subdomain:     subdomain,
		Path:          path,
		CustomDomains: custom,
		Notices:       siteNotices(facts, subdomain, path, custom, reachable),
	}
}

func siteNotices(facts SiteFacts, subdomain, path, custom ModeStatus, reachable bool) []Notice {
	var notices []Notice
	if !facts.Live {
		notices = append(notices, Notice{"no_live_version", "Nothing has been published to this site yet."})
	}
	switch {
	case subdomain.State == StateOff:
		notices = append(notices, Notice{"subdomain_off", "The subdomain address for this site is switched off."})
	case subdomain.Reason == ReasonWildcardFailed:
		notices = append(notices, Notice{"subdomain_unreliable", "The subdomain address for this site might not work right now."})
	}
	switch {
	case path.Reason == ReasonHTTPSRequired:
		notices = append(notices, Notice{"path_https_required", "The path address for this site is not available because the site is restricted and needs a secure connection."})
	case path.State == StateOff:
		notices = append(notices, Notice{"path_off", "The path address for this site is switched off."})
	}
	notices = append(notices, customDomainNotices(facts.Domains, custom)...)
	if facts.Live && !reachable {
		notices = append(notices, Notice{"unreachable", "This site cannot be opened at any address right now."})
	}
	return notices
}

func customDomainNotices(summary domains.Summary, custom ModeStatus) []Notice {
	switch {
	case summary.Total == 0:
		return nil
	case custom.State == StateOff:
		return []Notice{{"custom_domains_off", "Custom domains are switched off, so the custom domains for this site do not work."}}
	case summary.Verified < summary.Total:
		return []Notice{{"custom_domain_pending", "A custom domain is waiting for DNS verification."}}
	default:
		return nil
	}
}
