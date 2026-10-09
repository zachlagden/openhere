package api

import (
	"log/slog"
	"net/http"
	"net/netip"
	"path"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/invites"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/ratelimit"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const basePath = "/api/v1"

type Rule = ratelimit.Rule

type Limits struct {
	EmailTest     Rule
	InviteSend    Rule
	InviteLookup  Rule
	InviteAccept  Rule
	LoginPerIP    Rule
	LoginPerEmail Rule
	SetupPerIP    Rule
	SetupPerEmail Rule
	WildcardCheck Rule
	Handoff       Rule
}

func DefaultLimits() Limits {
	window := 15 * time.Minute
	return Limits{
		EmailTest:     Rule{Max: 5, Window: time.Minute},
		InviteSend:    Rule{Max: 30, Window: time.Hour},
		InviteLookup:  Rule{Max: 60, Window: time.Minute},
		InviteAccept:  Rule{Max: 10, Window: window},
		LoginPerIP:    Rule{Max: 30, Window: window},
		LoginPerEmail: Rule{Max: 10, Window: window},
		SetupPerIP:    Rule{Max: 10, Window: window},
		SetupPerEmail: Rule{Max: 5, Window: window},
		WildcardCheck: Rule{Max: 6, Window: time.Minute},
		Handoff:       Rule{Max: 30, Window: time.Minute},
	}
}

func (l Limits) withDefaults() Limits {
	defaults := DefaultLimits()
	fill := func(rule *Rule, fallback Rule) {
		if *rule == (Rule{}) {
			*rule = fallback
		}
	}
	fill(&l.EmailTest, defaults.EmailTest)
	fill(&l.InviteSend, defaults.InviteSend)
	fill(&l.InviteLookup, defaults.InviteLookup)
	fill(&l.InviteAccept, defaults.InviteAccept)
	fill(&l.WildcardCheck, defaults.WildcardCheck)
	fill(&l.Handoff, defaults.Handoff)
	return l
}

type Options struct {
	TrustedProxies []netip.Prefix
	Limits         Limits
	Now            func() time.Time
	Logger         *slog.Logger
	Public         config.PublicURL
	Wildcard       WildcardChecker
	Mailer         mail.Sender
	Invites        *invites.Service
}

type Server struct {
	auth       *auth.Service
	workspaces *workspaces.Service
	sites      *sites.Service
	publish    *publish.Service
	settings   *settings.Service
	access     *access.Service
	gate       *auth.SetupGate
	proxies    proxy.Trust
	logger     *slog.Logger
	public     config.PublicURL
	wildcard   WildcardChecker
	mailer     mail.Sender
	invites    *invites.Service
	now        func() time.Time

	inviteSend     *ratelimit.Limiter
	inviteLookup   *ratelimit.Limiter
	inviteAccept   *ratelimit.Limiter
	emailTestLimit *ratelimit.Limiter
	wildcardLimit  *ratelimit.Limiter
	handoffLimit   *ratelimit.Limiter
	loginIP        *ratelimit.Limiter
	loginEmail     *ratelimit.Limiter
	setupIP        *ratelimit.Limiter
	setupEmail     *ratelimit.Limiter
}

func New(svc *auth.Service, ws *workspaces.Service, siteSvc *sites.Service, publishSvc *publish.Service, settingsSvc *settings.Service, accessSvc *access.Service, gate *auth.SetupGate, opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	limits := opts.Limits
	if limits == (Limits{}) {
		limits = DefaultLimits()
	}
	limits = limits.withDefaults()
	mailer := opts.Mailer
	if mailer == nil {
		mailer = mail.Unconfigured{}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	limiter := func(r Rule) *ratelimit.Limiter { return ratelimit.New(r.Max, r.Window, opts.Now) }
	return &Server{
		auth:       svc,
		workspaces: ws,
		sites:      siteSvc,
		publish:    publishSvc,
		settings:   settingsSvc,
		access:     accessSvc,
		gate:       gate,
		proxies:    proxy.Trust{Prefixes: opts.TrustedProxies},
		logger:     logger,
		public:     opts.Public,
		wildcard:   opts.Wildcard,
		mailer:     mailer,
		invites:    opts.Invites,
		now:        now,

		inviteSend:     limiter(limits.InviteSend),
		inviteLookup:   limiter(limits.InviteLookup),
		inviteAccept:   limiter(limits.InviteAccept),
		emailTestLimit: limiter(limits.EmailTest),
		wildcardLimit:  limiter(limits.WildcardCheck),
		handoffLimit:   limiter(limits.Handoff),
		loginIP:        limiter(limits.LoginPerIP),
		loginEmail:     limiter(limits.LoginPerEmail),
		setupIP:        limiter(limits.SetupPerIP),
		setupEmail:     limiter(limits.SetupPerEmail),
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
	s.registerAccessRoutes(mux)
	s.registerAdminRoutes(mux)
	s.registerInviteRoutes(mux)

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
