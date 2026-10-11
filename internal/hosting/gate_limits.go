package hosting

import (
	"time"

	"github.com/zachlagden/openhere/internal/ratelimit"
)

type GateLimits struct {
	UnlockPerIP       ratelimit.Rule
	UnlockPerSite     ratelimit.Rule
	CodeRequestPerIP  ratelimit.Rule
	CodeRequestPerKey ratelimit.Rule
	CodeRequestSite   ratelimit.Rule
	CodeVerifyPerIP   ratelimit.Rule
	CodeVerifyPerKey  ratelimit.Rule
	CodeVerifySite    ratelimit.Rule
}

func DefaultGateLimits() GateLimits {
	quarter := 15 * time.Minute
	return GateLimits{
		UnlockPerIP:       ratelimit.Rule{Max: 10, Window: quarter},
		UnlockPerSite:     ratelimit.Rule{Max: 100, Window: quarter},
		CodeRequestPerIP:  ratelimit.Rule{Max: 10, Window: quarter},
		CodeRequestPerKey: ratelimit.Rule{Max: 3, Window: 10 * time.Minute},
		CodeRequestSite:   ratelimit.Rule{Max: 60, Window: time.Hour},
		CodeVerifyPerIP:   ratelimit.Rule{Max: 20, Window: quarter},
		CodeVerifyPerKey:  ratelimit.Rule{Max: 10, Window: quarter},
		CodeVerifySite:    ratelimit.Rule{Max: 200, Window: quarter},
	}
}

type gateLimiters struct {
	unlockIP, unlockSite  *ratelimit.Limiter
	requestIP, requestKey *ratelimit.Limiter
	requestSite           *ratelimit.Limiter
	verifyIP, verifyKey   *ratelimit.Limiter
	verifySite            *ratelimit.Limiter
}

func newGateLimiters(l GateLimits, now func() time.Time) gateLimiters {
	build := func(r ratelimit.Rule) *ratelimit.Limiter { return ratelimit.New(r.Max, r.Window, now) }
	return gateLimiters{
		unlockIP: build(l.UnlockPerIP), unlockSite: build(l.UnlockPerSite),
		requestIP: build(l.CodeRequestPerIP), requestKey: build(l.CodeRequestPerKey), requestSite: build(l.CodeRequestSite),
		verifyIP: build(l.CodeVerifyPerIP), verifyKey: build(l.CodeVerifyPerKey), verifySite: build(l.CodeVerifySite),
	}
}
