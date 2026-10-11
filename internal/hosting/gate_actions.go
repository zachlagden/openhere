package hosting

import (
	"net/http"

	"github.com/zachlagden/openhere/internal/access"
)

const (
	wrongPasswordMessage = "That password is not right."
	wrongCodeMessage     = "That code is not right or has expired."
	codeSentMessage      = "If that address is allowed, a code is on its way. It expires in 10 minutes."
	noEmailMessage       = "Email sign-in is not available on this instance."
	staleLinkMessage     = "That sign-in link has expired. Start again."
)

func (g *gate) grant(req siteRequest, state access.State, method access.Method, subject, returnPath string) {
	value, _, err := g.access.IssueCookie(req.r.Context(), req.site.ID, state.Epoch, method, subject)
	if err != nil {
		g.fail(req.w, err)
		return
	}
	g.setViewerCookie(req, value)
	req.redirectTo(returnPathFrom(returnPath))
}

func (g *gate) unlock(req siteRequest, state access.State) {
	values, ok := g.readForm(req)
	if !ok {
		return
	}
	if !g.within(req, state, g.limits.unlockIP, g.clientKey(req.r)) || !g.within(req, state, g.limits.unlockSite, req.site.ID) {
		return
	}
	if !g.checkForm(req, state, values) {
		return
	}
	fresh, matched, err := g.access.CheckPassword(req.r.Context(), req.site.ID, values.Get("password"))
	if err != nil {
		g.fail(req.w, err)
		return
	}
	if !matched {
		g.renderPage(req, fresh, pageData{Status: http.StatusUnauthorized, Message: wrongPasswordMessage, Return: values.Get("return")})
		return
	}
	g.grant(req, fresh, access.MethodPassword, "", values.Get("return"))
}

func (g *gate) requestCode(req siteRequest, state access.State) {
	values, ok := g.readForm(req)
	if !ok {
		return
	}
	email := values.Get("email")
	if !g.withinCodeRequest(req, state, email) || !g.checkForm(req, state, values) {
		return
	}
	if state.Mode != access.ModeRestricted {
		g.renderPage(req, state, pageData{Status: http.StatusUnauthorized, Return: values.Get("return")})
		return
	}
	if !g.access.EmailCodeAvailable() {
		g.renderPage(req, state, pageData{Status: http.StatusOK, Message: noEmailMessage, Info: true, Return: values.Get("return")})
		return
	}
	in := access.CodeRequest{SiteID: req.site.ID, SiteLabel: req.site.Slug, Email: email}
	if err := g.access.RequestCode(req.r.Context(), in); err != nil {
		g.fail(req.w, err)
		return
	}
	g.renderPage(req, state, pageData{
		Kind: pageCodeEntry, Status: http.StatusOK, Message: codeSentMessage, Info: true, Email: email, Return: values.Get("return"),
	})
}

func (g *gate) withinCodeRequest(req siteRequest, state access.State, email string) bool {
	return g.within(req, state, g.limits.requestIP, g.clientKey(req.r)) &&
		g.within(req, state, g.limits.requestSite, req.site.ID) &&
		g.within(req, state, g.limits.requestKey, emailKey(req.site.ID, email))
}

func (g *gate) verifyCode(req siteRequest, state access.State) {
	values, ok := g.readForm(req)
	if !ok {
		return
	}
	email := values.Get("email")
	if !g.withinCodeVerify(req, state, email) || !g.checkForm(req, state, values) {
		return
	}
	fresh, matched, err := g.access.VerifyCode(req.r.Context(), req.site.ID, email, values.Get("code"))
	if err != nil {
		g.fail(req.w, err)
		return
	}
	if !matched {
		g.renderPage(req, fresh, pageData{
			Kind: pageCodeEntry, Status: http.StatusUnauthorized, Message: wrongCodeMessage, Email: email, Return: values.Get("return"),
		})
		return
	}
	g.grant(req, fresh, access.MethodEmail, "", values.Get("return"))
}

func (g *gate) withinCodeVerify(req siteRequest, state access.State, email string) bool {
	return g.within(req, state, g.limits.verifyIP, g.clientKey(req.r)) &&
		g.within(req, state, g.limits.verifySite, req.site.ID) &&
		g.within(req, state, g.limits.verifyKey, emailKey(req.site.ID, email))
}

func (g *gate) callback(req siteRequest, state access.State) {
	query := req.r.URL.Query()
	fresh, userID, ok, err := g.access.ConsumeHandoff(req.r.Context(), req.site.ID, query.Get("token"))
	if err != nil {
		g.fail(req.w, err)
		return
	}
	if !ok {
		g.renderPage(req, state, pageData{Status: http.StatusUnauthorized, Message: staleLinkMessage, Return: query.Get("return")})
		return
	}
	g.grant(req, fresh, access.MethodMember, userID, query.Get("return"))
}
