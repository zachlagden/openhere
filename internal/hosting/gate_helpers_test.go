package hosting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	otherHost    = "other.sites.example.com"
	sharedSecret = "a shared secret"
)

var (
	tokenPattern = regexp.MustCompile(`name="token" value="([^"]+)"`)
	codePattern  = regexp.MustCompile(`\b[0-9]{8}\b`)
)

func strptr(s string) *string { return &s }

func listptr(values ...string) *[]string {
	if values == nil {
		values = []string{}
	}
	return &values
}

func passwordMode() access.ConfigUpdate {
	secret := sharedSecret
	return access.ConfigUpdate{Mode: strptr("password"), Password: access.PasswordChange{Set: true, Value: &secret}}
}

func restrictedMode(emails ...string) access.ConfigUpdate {
	return access.ConfigUpdate{Mode: strptr("restricted"), Emails: listptr(emails...)}
}

func (e *env) siteIDOf(slug string) string {
	e.t.Helper()
	var id string
	row := e.db.QueryRowContext(context.Background(), "SELECT id FROM sites WHERE slug = $1", slug)
	if err := row.Scan(&id); err != nil {
		e.t.Fatalf("site %s: %v", slug, err)
	}
	return id
}

func (e *env) protect(slug string, update access.ConfigUpdate) {
	e.t.Helper()
	if _, err := e.access.Update(context.Background(), e.siteIDOf(slug), update); err != nil {
		e.t.Fatalf("protect %s: %v", slug, err)
	}
}

func (e *env) gatedDemo(update access.ConfigUpdate) {
	e.t.Helper()
	e.publishSite("demo", nextExport)
	e.enableSubdomains()
	e.protect("demo", update)
}

func (e *env) postForm(host, target string, form url.Values, remote string, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.t.Context(), http.MethodPost, "http://"+host+target, strings.NewReader(form.Encode()))
	req.Host = host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if remote != "" {
		req.RemoteAddr = remote
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func tokenIn(t *testing.T, body string) string {
	t.Helper()
	match := tokenPattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("no form token in %q", body)
	}
	return match[1]
}

func (e *env) formToken(host, target string) string {
	e.t.Helper()
	return tokenIn(e.t, e.get(host, target).Body.String())
}

func (e *env) unlock(host, prefix, password string) *httptest.ResponseRecorder {
	e.t.Helper()
	form := url.Values{"password": {password}, "token": {e.formToken(host, prefix+"/")}, "return": {"/about"}}
	return e.postForm(host, prefix+UnlockPath, form, "")
}

func viewerCookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if strings.HasSuffix(c.Name, viewerCookie) {
			return c
		}
	}
	return nil
}

func cookieHeader(c *http.Cookie) string {
	return c.Name + "=" + c.Value
}

func (e *env) unlockedCookie(host, prefix string) *http.Cookie {
	e.t.Helper()
	rec := e.unlock(host, prefix, sharedSecret)
	cookie := viewerCookieOf(rec)
	if rec.Code != http.StatusSeeOther || cookie == nil {
		e.t.Fatalf("unlock failed: %d %q", rec.Code, rec.Body.String())
	}
	return cookie
}

func (e *env) withCookie(host, target string, c *http.Cookie, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.get(host, target, append([]string{"Cookie", cookieHeader(c)}, headers...)...)
}

func lastCodeIn(t *testing.T, e *env) string {
	t.Helper()
	messages := e.mailer.Messages()
	if len(messages) == 0 {
		t.Fatal("no mail was recorded")
	}
	code := codePattern.FindString(messages[len(messages)-1].Text)
	if code == "" {
		t.Fatalf("no code in %q", messages[len(messages)-1].Text)
	}
	return code
}

func (e *env) requestCodeFor(email string) *httptest.ResponseRecorder {
	e.t.Helper()
	form := url.Values{"email": {email}, "token": {e.formToken(siteHost, "/")}, "return": {"/about"}}
	return e.postForm(siteHost, CodePath, form, "")
}

func (e *env) verifyCodeFor(code string) *httptest.ResponseRecorder {
	e.t.Helper()
	form := url.Values{"email": {"sam@example.com"}, "code": {code}, "token": {e.formToken(siteHost, "/")}, "return": {"/about"}}
	return e.postForm(siteHost, VerifyPath, form, "")
}

func hasFrameAncestorsNone(h http.Header) bool {
	return strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
}

func proxyTrust(cidrs ...string) proxy.Trust {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefixes = append(prefixes, netip.MustParsePrefix(cidr))
	}
	return proxy.Trust{Prefixes: prefixes}
}

func (e *env) tokenFor(host, remote string) string {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.t.Context(), http.MethodGet, "http://"+host+"/", nil)
	req.Host = host
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return tokenIn(e.t, rec.Body.String())
}

func (e *env) user(email string) string {
	e.t.Helper()
	id := storetest.InsertUser(e.t, e.db, email)
	if _, err := workspaces.CreatePersonal(context.Background(), e.db, id, e.clock.Now()); err != nil {
		e.t.Fatalf("CreatePersonal: %v", err)
	}
	return id
}

func (e *env) sharedWorkspace() string {
	e.t.Helper()
	if _, err := e.workspaces.CreateShared(context.Background(), e.alex, "Acme Ltd", "acme"); err != nil {
		e.t.Fatalf("CreateShared: %v", err)
	}
	return "acme"
}

func (e *env) grantSite(email, slug string) {
	e.t.Helper()
	site, err := e.sites.GetSite(context.Background(), e.alex, slug)
	if err != nil {
		e.t.Fatal(err)
	}
	in := workspaces.AddMemberInput{Email: email, Role: "read", ScopeType: "site", ScopeID: site.ID}
	if _, err := e.workspaces.AddMember(context.Background(), e.alex, "acme", in); err != nil {
		e.t.Fatalf("grant: %v", err)
	}
}
