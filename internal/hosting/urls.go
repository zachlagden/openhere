package hosting

import (
	"github.com/zachlagden/openhere/internal/config"
	"github.com/zachlagden/openhere/internal/domains"
	"github.com/zachlagden/openhere/internal/settings"
)

const pathModePrefix = "/s/"

func SiteURL(public config.PublicURL, cfg settings.Settings, slug string) string {
	if cfg.SubdomainEnabled && cfg.BaseDomain != "" {
		withBase := config.PublicURL{Scheme: public.Scheme, Host: slug + "." + cfg.BaseDomain, Port: public.Port}
		return withBase.String() + "/"
	}
	if cfg.PathEnabled {
		return public.String() + pathModePrefix + slug + "/"
	}
	return ""
}

func CustomDomainURL(public config.PublicURL, host string) string {
	return config.PublicURL{Scheme: public.Scheme, Host: host}.String() + "/"
}

func PreferredURL(public config.PublicURL, cfg settings.Settings, slug string, custom domains.Summary) string {
	if cfg.CustomDomainsEnabled && custom.Primary != "" {
		return CustomDomainURL(public, custom.Primary)
	}
	if address := SiteURL(public, cfg, slug); address != "" {
		return address
	}
	if cfg.CustomDomainsEnabled && custom.Fallback != "" {
		return CustomDomainURL(public, custom.Fallback)
	}
	return ""
}
