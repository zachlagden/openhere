package hosting

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/store"
)

type ProbeConsumer interface {
	Consume(token string) (string, bool)
}

type Options struct {
	Public   config.PublicURL
	Probes   ProbeConsumer
	Resolver *Resolver
	Logger   *slog.Logger
	Access   *access.Service
	Proxies  proxy.Trust
	Limits   *GateLimits
	Now      func() time.Time
}

type Router struct {
	management http.Handler
	settings   *settings.Service
	resolver   *Resolver
	server     siteServer
	probes     ProbeConsumer
	gate       *gate
	host       string
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
		server:     siteServer{files: files, index: fileIndex{db: db}, logger: logger},
		probes:     opts.Probes,
		host:       opts.Public.Host,
		logger:     logger,
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if NormaliseHost(r.Host) != rt.host {
		rt.serveSiteHost(w, r)
		return
	}
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
	rt.serveSite(w, r, slugPart, "/"+tail, pathModePrefix+slugPart)
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
	slugPart, ok := SubdomainSlug(NormaliseHost(r.Host), cfg.BaseDomain)
	if !cfg.SubdomainEnabled || !ok {
		writeNotFound(w, r)
		return
	}
	rt.serveSite(w, r, slugPart, r.URL.Path, "")
}

func methodAllowed(r *http.Request, requestPath string) bool {
	return isReadMethod(r.Method) || (r.Method == http.MethodPost && isGatePost(requestPath))
}

func (rt *Router) serveSite(w http.ResponseWriter, r *http.Request, slugPart, requestPath, prefix string) {
	site, live, err := rt.resolver.Lookup(r.Context(), slugPart)
	if err != nil {
		rt.fail(w, err)
		return
	}
	if !live {
		writeNotFound(w, r)
		return
	}
	verdict := rt.gate.evaluate(siteRequest{w: w, r: r, site: site, path: requestPath, prefix: prefix})
	if verdict == outcomeHandled {
		return
	}
	if !isReadMethod(r.Method) {
		writeMethodNotAllowed(w)
		return
	}
	rt.server.serve(w, r, site, requestPath, serveTarget{prefix: prefix, private: verdict == outcomeOpenPrivate})
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
