package hosting

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"

	"github.com/zachlagden/openhere/internal/access"
)

type pageKind int

const (
	pageByMode pageKind = iota
	pageCodeEntry
	pageHTTPSRequired
)

type pageData struct {
	Kind    pageKind
	Status  int
	Message string
	Info    bool
	Email   string
	Return  string
}

type pageView struct {
	Title      string
	Heading    string
	Intro      string
	Message    string
	Info       bool
	Token      string
	Return     string
	Email      string
	Password   bool
	SignIn     bool
	EmailForm  bool
	NoEmail    bool
	CodeEntry  bool
	UnlockURL  string
	CodeURL    string
	VerifyURL  string
	HandoffURL string
}

var pageTemplate = template.Must(template.New("gate").Parse(gatePageHTML))

func (g *gate) renderPage(req siteRequest, state access.State, data pageData) {
	view, err := g.buildView(req, state, data)
	if err != nil {
		g.fail(req.w, err)
		return
	}
	var body bytes.Buffer
	if err := pageTemplate.Execute(&body, view); err != nil {
		g.fail(req.w, err)
		return
	}
	status := data.Status
	if status == 0 {
		status = http.StatusUnauthorized
	}
	setGatedHeaders(req.w, req.pathMode(), true)
	h := req.w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", cacheGatePage)
	req.w.WriteHeader(status)
	if req.r.Method == http.MethodHead {
		return
	}
	if _, err := req.w.Write(body.Bytes()); err != nil {
		logWriteFailure(err)
	}
}

func (g *gate) buildView(req siteRequest, state access.State, data pageData) (pageView, error) {
	returnPath := returnFor(req, data)
	view := pageView{
		Title: "Protected site", Heading: "This site is protected", Message: data.Message, Info: data.Info, Return: returnPath, Email: data.Email,
		UnlockURL: req.prefix + UnlockPath, CodeURL: req.prefix + CodePath, VerifyURL: req.prefix + VerifyPath,
		HandoffURL: g.handoffURL(req, returnPath),
	}
	if data.Kind == pageHTTPSRequired {
		view.Heading = "HTTPS is required"
		view.Intro = "This site needs a sign-in, and sign-in only works over a secure connection. Open it over HTTPS, or ask the site owner to use subdomain hosting."
		return view, nil
	}
	token, err := g.access.FormToken(req.r.Context(), req.site.ID, state.Epoch, g.clientKey(req.r))
	if err != nil {
		return pageView{}, err
	}
	view.Token = token
	g.chooseForms(&view, state, data.Kind)
	return view, nil
}

func (g *gate) chooseForms(view *pageView, state access.State, kind pageKind) {
	if state.Mode == access.ModePassword {
		view.Heading = "This site needs a password"
		view.Password = true
		return
	}
	view.Heading = "This site is private"
	view.SignIn = true
	switch {
	case kind == pageCodeEntry:
		view.CodeEntry = true
	case g.access.EmailCodeAvailable():
		view.EmailForm = true
	default:
		view.NoEmail = true
	}
}

func (g *gate) handoffURL(req siteRequest, returnPath string) string {
	query := url.Values{"site": {req.site.Slug}, "return": {returnPath}}
	if req.customHost != "" {
		query.Set("host", req.customHost)
	}
	return g.public.String() + HandoffPath + "?" + query.Encode()
}

func returnFor(req siteRequest, data pageData) string {
	if data.Return != "" {
		return returnPathFrom(data.Return)
	}
	if req.path == CallbackPath || isGatePost(req.path) {
		return "/"
	}
	return returnPathFrom(req.returnTarget())
}
