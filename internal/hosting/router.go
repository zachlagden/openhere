package hosting

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/httptimeouts"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/store"
)

type ProbeConsumer interface {
	Consume(token string) (string, bool)
}

type DomainLookup interface {
	Lookup(ctx context.Context, host string) (domains.Target, bool, error)
}

type Options struct {
	Public   config.PublicURL
	Probes   ProbeConsumer
	Resolver *Resolver
	Logger   *slog.Logger
	Access   *access.Service
	Domains  DomainLookup
	Proxies  proxy.Trust
	Limits   *GateLimits
	Views    ViewRecorder
	Timeouts httptimeouts.Timeouts
	HSTS     bool
	Now      func() time.Time
}

type Router struct {
	management http.Handler
	settings   *settings.Service
	resolver   *Resolver
	server     siteServer
	probes     ProbeConsumer
	domains    DomainLookup
	gate       *gate
	host       string
	hsts       bool
	logger     *slog.Logger
}

func New(management http.Handler, settingsSvc *settings.Service, db store.Store, files *sitefiles.Store, opts Options) *Router {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	resolver := opts.Resolver
	if resolver == nil {
		resolver = NewResolver(db, nil)
	}
	return &Router{
		gate:       newGate(db, opts, logger),
		management: management,
		settings:   settingsSvc,
		resolver:   resolver,
		server:     siteServer{files: files, index: fileIndex{db: db}, logger: logger, views: opts.Views, proxies: opts.Proxies, timeouts: opts.Timeouts.OrDefaults()},
		probes:     opts.Probes,
		domains:    opts.Domains,
		host:       opts.Public.Host,
		hsts:       opts.HSTS,
		logger:     logger,
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if NormaliseHost(r.Host) != rt.host {
		rt.serveSiteHost(w, r)
		return
	}
	rt.setHSTS(w, r)
	if r.URL.Path == "/s" || strings.HasPrefix(r.URL.Path, pathModePrefix) {
		rt.servePathMode(w, r)
		return
	}
	rt.management.ServeHTTP(w, r)
}

func (rt *Router) servePathMode(w http.ResponseWriter, r *http.Request) {
	setSiteHeaders(w, true)
	rest, _ := strings.CutPrefix(r.URL.Path, pathModePrefix)
	slugPart, tail, hasSlash := strings.Cut(rest, "/")
	if !methodAllowed(r, "/"+tail) {
		writeMethodNotAllowed(w)
		return
	}
	cfg, err := rt.settings.Get(r.Context())
	if err != nil {
		rt.fail(w, err)
		return
	}
	if !cfg.PathEnabled || !validSlug(slugPart) {
		writeNotFound(w, r)
		return
	}
	if !hasSlash {
		redirectToDirectory(w, r, r.URL.Path+"/")
		return
	}
	rt.serveSite(w, r, siteTarget{slug: slugPart, path: "/" + tail, prefix: pathModePrefix + slugPart})
}

func (rt *Router) serveSiteHost(w http.ResponseWriter, r *http.Request) {
	setSiteHeaders(w, false)
	if !methodAllowed(r, r.URL.Path) {
		writeMethodNotAllowed(w)
		return
	}
	if rt.answerProbe(w, r) {
		return
	}
	cfg, err := rt.settings.Get(r.Context())
	if err != nil {
		rt.fail(w, err)
		return
	}
	host := NormaliseHost(r.Host)
	if slugPart, ok := SubdomainSlug(host, cfg.BaseDomain); ok {
		if !cfg.SubdomainEnabled {
			writeNotFound(w, r)
			return
		}
		rt.setHSTS(w, r)
		rt.serveSite(w, r, siteTarget{slug: slugPart, path: r.URL.Path})
		return
	}
	rt.serveCustomDomain(w, r, host, cfg)
}

func (rt *Router) serveCustomDomain(w http.ResponseWriter, r *http.Request, host string, cfg settings.Settings) {
	if !cfg.CustomDomainsEnabled || rt.domains == nil {
		writeNotFound(w, r)
		return
	}
	target, serves, err := rt.domains.Lookup(r.Context(), host)
	if err != nil {
		rt.fail(w, err)
		return
	}
	if !serves {
		writeNotFound(w, r)
		return
	}
	rt.setHSTS(w, r)
	rt.serveSite(w, r, siteTarget{slug: target.Slug, path: r.URL.Path, customHost: host})
}

func methodAllowed(r *http.Request, requestPath string) bool {
	return isReadMethod(r.Method) || (r.Method == http.MethodPost && isGatePost(requestPath))
}

type siteTarget struct {
	slug       string
	path       string
	prefix     string
	customHost string
}

func (rt *Router) serveSite(w http.ResponseWriter, r *http.Request, target siteTarget) {
	site, live, err := rt.resolver.Lookup(r.Context(), target.slug)
	if err != nil {
		rt.fail(w, err)
		return
	}
	if !live {
		writeNotFound(w, r)
		return
	}
	verdict := rt.gate.evaluate(siteRequest{w: w, r: r, site: site, path: target.path, prefix: target.prefix, customHost: target.customHost})
	if verdict == outcomeHandled {
		return
	}
	if !isReadMethod(r.Method) {
		writeMethodNotAllowed(w)
		return
	}
	rt.server.serve(w, r, site, target.path, serveTarget{prefix: target.prefix, private: verdict == outcomeOpenPrivate})
}

func (rt *Router) answerProbe(w http.ResponseWriter, r *http.Request) bool {
	token, isProbe := strings.CutPrefix(r.URL.Path, ProbePath)
	if !isProbe || rt.probes == nil {
		return false
	}
	secret, accepted := rt.probes.Consume(token)
	if token == "" || !accepted {
		writeNotFound(w, r)
		return true
	}
	writeText(w, http.StatusOK, secret, r.Method != http.MethodHead)
	return true
}

func (rt *Router) fail(w http.ResponseWriter, err error) {
	rt.logger.Error("route request", "error", err)
	writeServerError(w)
}
