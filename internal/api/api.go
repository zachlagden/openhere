package api

import (
	"log/slog"
	"net/http"
	"net/netip"
	"path"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/ratelimit"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const basePath = "/api/v1"

type Rule struct {
	Max    int
	Window time.Duration
}

type Limits struct {
	LoginPerIP    Rule
	LoginPerEmail Rule
	SetupPerIP    Rule
	SetupPerEmail Rule
	WildcardCheck Rule
}

func DefaultLimits() Limits {
	window := 15 * time.Minute
	return Limits{
		LoginPerIP:    Rule{Max: 30, Window: window},
		LoginPerEmail: Rule{Max: 10, Window: window},
		SetupPerIP:    Rule{Max: 10, Window: window},
		SetupPerEmail: Rule{Max: 5, Window: window},
		WildcardCheck: Rule{Max: 6, Window: time.Minute},
	}
}

type Options struct {
	TrustedProxies []netip.Prefix
	Limits         Limits
	Now            func() time.Time
	Logger         *slog.Logger
	Public         config.PublicURL
	Wildcard       WildcardChecker
}

type Server struct {
	auth       *auth.Service
	workspaces *workspaces.Service
	sites      *sites.Service
	publish    *publish.Service
	settings   *settings.Service
	gate       *auth.SetupGate
	proxies    proxyTrust
	logger     *slog.Logger
	public     config.PublicURL
	wildcard   WildcardChecker

	wildcardLimit *ratelimit.Limiter
	loginIP       *ratelimit.Limiter
	loginEmail    *ratelimit.Limiter
	setupIP       *ratelimit.Limiter
	setupEmail    *ratelimit.Limiter
}

func New(svc *auth.Service, ws *workspaces.Service, siteSvc *sites.Service, publishSvc *publish.Service, settingsSvc *settings.Service, gate *auth.SetupGate, opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	limits := opts.Limits
	if limits == (Limits{}) {
		limits = DefaultLimits()
	}
	if limits.WildcardCheck == (Rule{}) {
		limits.WildcardCheck = DefaultLimits().WildcardCheck
	}
	limiter := func(r Rule) *ratelimit.Limiter { return ratelimit.New(r.Max, r.Window, opts.Now) }
	return &Server{
		auth:       svc,
		workspaces: ws,
		sites:      siteSvc,
		publish:    publishSvc,
		settings:   settingsSvc,
		gate:       gate,
		proxies:    proxyTrust{prefixes: opts.TrustedProxies},
		logger:     logger,
		public:     opts.Public,
		wildcard:   opts.Wildcard,

		wildcardLimit: limiter(limits.WildcardCheck),
		loginIP:       limiter(limits.LoginPerIP),
		loginEmail:    limiter(limits.LoginPerEmail),
		setupIP:       limiter(limits.SetupPerIP),
		setupEmail:    limiter(limits.SetupPerEmail),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+basePath+"/setup/status", s.identify(http.HandlerFunc(s.handleSetupStatus)))
	mux.Handle("POST "+basePath+"/setup", s.identify(http.HandlerFunc(s.handleSetup)))
	mux.Handle("POST "+basePath+"/auth/login", s.identify(http.HandlerFunc(s.handleLogin)))

	mux.Handle("POST "+basePath+"/auth/logout", s.protected(s.handleLogout))
	mux.Handle("GET "+basePath+"/me", s.protected(s.handleMe))
	mux.Handle("GET "+basePath+"/me/keys", s.protected(s.handleListKeys))
	mux.Handle("POST "+basePath+"/me/keys", s.protected(s.handleCreateKey))
	mux.Handle("DELETE "+basePath+"/me/keys/{id}", s.protected(s.handleRevokeKey))

	s.registerWorkspaceRoutes(mux)
	s.registerSiteRoutes(mux)
	s.registerPublishRoutes(mux)
	s.registerSettingsRoutes(mux)
	s.registerHostingRoutes(mux)

	notFound := s.protected(s.handleNotFound)
	mux.Handle("/", notFound)
	return securityHeaders(rejectUncleanPaths(mux, notFound))
}

func rejectUncleanPaths(next, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		cleaned := path.Clean(p)
		if p != cleaned && (cleaned == "/" || p != cleaned+"/") {
			notFound.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, codeNotFound, "Not found.")
}
