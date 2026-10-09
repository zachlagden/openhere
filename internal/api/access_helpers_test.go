package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	siteSecret = "a shared secret"
	evilSecret = "hunter2-should-never-leak"
)

type accessSetup struct {
	h      *harness
	alex   actor
	sam    actor
	kim    actor
	lee    actor
	siteID string
}

func newAccessSetup(t *testing.T, h *harness) accessSetup {
	t.Helper()
	alex := h.rootActor()
	alex.createWorkspace("Acme Ltd", "acme")
	sam := h.bearerActor("sam@example.com")
	kim := h.bearerActor("kim@example.com")
	lee := h.bearerActor("lee@example.com")
	alex.addMember("acme", addMemberRequest{Email: "sam@example.com", Role: "read"})
	alex.addMember("acme", addMemberRequest{Email: "kim@example.com", Role: "write"})
	alex.publishLive("guarded", siteFiles{"index.html": "<h1>secret home</h1>", "about.html": "about"})
	site := decodeBody[siteEnvelope](t, alex.expect(http.StatusOK, "GET", "/api/v1/sites/guarded", nil)).Site
	return accessSetup{h: h, alex: alex, sam: sam, kim: kim, lee: lee, siteID: site.ID}
}

func (a actor) access() accessResponse {
	a.h.t.Helper()
	return decodeBody[accessEnvelope](a.h.t, a.expect(http.StatusOK, "GET", "/api/v1/publish/guarded/access", nil)).Access
}

func (a actor) setAccess(body any) accessResponse {
	a.h.t.Helper()
	return decodeBody[accessEnvelope](a.h.t, a.expect(http.StatusOK, "PATCH", "/api/v1/publish/guarded/access", body)).Access
}

func (h *harness) sessionFor(email string) *http.Cookie {
	h.t.Helper()
	rec := h.do("POST", "/api/v1/auth/login", loginRequest{Email: email, Password: testPassword})
	expectStatus(h.t, rec, http.StatusOK)
	cookie := sessionCookie(rec)
	if cookie == nil {
		h.t.Fatal("login set no session cookie")
	}
	return cookie
}

func (h *harness) routedGet(host, target string, headers ...string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequestWithContext(h.t.Context(), http.MethodGet, "http://"+host+target, nil)
	req.Host = host
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.routed.ServeHTTP(rec, req)
	return rec
}

func locationPath(t *testing.T, rec *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad Location %q: %v", rec.Header().Get("Location"), err)
	}
	return parsed
}

func viewerCookieFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if strings.HasSuffix(c.Name, "viewer") {
			return c
		}
	}
	return nil
}
