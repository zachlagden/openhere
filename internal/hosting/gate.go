package hosting

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/store"
)

type outcome int

const (
	outcomeHandled outcome = iota
	outcomeOpen
	outcomeOpenPrivate
)

type gate struct {
	access  *access.Service
	public  config.PublicURL
	proxies proxy.Trust
	limits  gateLimiters
	logger  *slog.Logger
}

func (g *gate) evaluate(req siteRequest) outcome {
	decision, err := g.access.Check(req.r.Context(), req.site.ID, g.viewerCookies(req))
	if errors.Is(err, access.ErrSiteNotFound) {
		writeNotFound(req.w, req.r)
		return outcomeHandled
	}
	if err != nil {
		g.fail(req.w, err)
		return outcomeHandled
	}
	if decision.Mode == access.ModePublic {
		return outcomeOpen
	}
	setGatedHeaders(req.w, req.pathMode(), false)
	if g.handleRoute(req, decision) {
		return outcomeHandled
	}
	if decision.Allowed {
		return outcomeOpenPrivate
	}
	g.deny(req, decision.State, pageData{})
	return outcomeHandled
}

func (g *gate) handleRoute(req siteRequest, decision access.Decision) bool {
	if !g.canHoldCookies(req) {
		g.deny(req, decision.State, pageData{})
		return true
	}
	switch {
	case req.r.Method == http.MethodGet && req.path == CallbackPath:
		g.callback(req, decision.State)
	case req.r.Method == http.MethodPost && req.path == UnlockPath:
		g.unlock(req, decision.State)
	case req.r.Method == http.MethodPost && req.path == CodePath:
		g.requestCode(req, decision.State)
	case req.r.Method == http.MethodPost && req.path == VerifyPath:
		g.verifyCode(req, decision.State)
	default:
		return false
	}
	return true
}

func (g *gate) canHoldCookies(req siteRequest) bool {
	return !req.pathMode() || g.secure(req.r)
}

func (g *gate) deny(req siteRequest, state access.State, data pageData) {
	if !g.canHoldCookies(req) {
		g.renderPage(req, state, pageData{Kind: pageHTTPSRequired, Status: http.StatusForbidden})
		return
	}
	if !isNavigation(req.r) {
		writeUnauthorized(req.w, req.r)
		return
	}
	if data.Status == 0 {
		data.Status = http.StatusUnauthorized
	}
	g.renderPage(req, state, data)
}

func (g *gate) fail(w http.ResponseWriter, err error) {
	g.logger.Error("check site access", "error", err)
	writeServerError(w)
}

func newGate(db store.Store, opts Options, logger *slog.Logger) *gate {
	svc := opts.Access
	if svc == nil {
		svc = access.New(db, access.Options{Logger: logger})
	}
	limits := DefaultGateLimits()
	if opts.Limits != nil {
		limits = *opts.Limits
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &gate{
		access: svc, public: opts.Public, proxies: opts.Proxies, limits: newGateLimiters(limits, now), logger: logger,
	}
}
