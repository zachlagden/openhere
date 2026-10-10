package hosting

import (
	"github.com/zachlagden/selfhost/internal/config"
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
	Slug  string
	Live  bool
	Gated bool
}

type SiteAvailability struct {
	Reachable bool
	URL       string
	Subdomain ModeStatus
	Path      ModeStatus
	Notices   []Notice
}

type InstanceAvailability struct {
	Subdomain ModeStatus
	Path      ModeStatus
	Notices   []Notice
	Degraded  bool
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
	return InstanceAvailability{
		Subdomain: subdomain, Path: path, Notices: notices,
		Degraded: subdomain.Reason == ReasonWildcardFailed,
	}
}

func SiteHosting(public config.PublicURL, cfg settings.Settings, facts SiteFacts) SiteAvailability {
	subdomain, path := subdomainStatus(cfg), pathStatus(cfg)
	if path.State == StateOn && facts.Gated && !public.Secure() {
		path = ModeStatus{StateOff, ReasonHTTPSRequired}
	}
	reachable := facts.Live && (subdomain.State == StateOn || path.State == StateOn)
	return SiteAvailability{
		Reachable: reachable,
		URL:       SiteURL(public, cfg, facts.Slug),
		Subdomain: subdomain,
		Path:      path,
		Notices:   siteNotices(facts, subdomain, path, reachable),
	}
}

func siteNotices(facts SiteFacts, subdomain, path ModeStatus, reachable bool) []Notice {
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
	if facts.Live && !reachable {
		notices = append(notices, Notice{"unreachable", "This site cannot be opened at any address right now."})
	}
	return notices
}
