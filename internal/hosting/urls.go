package hosting

import (
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/settings"
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
