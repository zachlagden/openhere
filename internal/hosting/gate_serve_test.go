package hosting

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/access"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

var secretContent = []string{"home", "about", "docs index", "custom missing", "console.log", "deep"}

var handoffReturn = regexp.MustCompile(`return=[^&"]*`)

var returnField = regexp.MustCompile(`name="return" value="[^"]*"`)

func assertNoContent(t *testing.T, label, body string) {
	t.Helper()
	body = returnField.ReplaceAllString(body, "")
	body = handoffReturn.ReplaceAllString(body, "")
	for _, secret := range secretContent {
		if strings.Contains(body, secret) {
			t.Errorf("%s: the response leaks %q: %s", label, secret, body)
		}
	}
}

func TestPublicSitesStayEmbeddableAndShared(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()
		rec := e.site("/")
		h := rec.Header()
		if h.Get("Content-Security-Policy") != "" || h.Get("X-Frame-Options") != "" {
			t.Errorf("a public site refuses framing: %v", h)
		}
		if h.Get("Cache-Control") != CacheHTML {
			t.Errorf("Cache-Control = %q", h.Get("Cache-Control"))
		}
		if got := e.get(managementHost, "/s/demo/").Header().Get("Content-Security-Policy"); got != wantSandbox {
			t.Errorf("path-mode public policy = %q", got)
		}
	})
}

func TestGatedSitesAnswerAnonymousNavigationsWithAGatePage(t *testing.T) {
	modes := map[string]access.ConfigUpdate{"password": passwordMode(), "restricted": restrictedMode("sam@example.com")}
	for name, update := range modes {
		storetest.Each(t, func(t *testing.T, db store.Store) {
			e := newEnv(t, db)
			e.gatedDemo(update)
			for _, target := range []string{"/", "/about", "/missing", "/docs/", "/_next/static/chunk-1.js", "/404.html", "/about?x=1"} {
				rec := e.site(target)
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s %s: status = %d", name, target, rec.Code)
				}
				assertNoContent(t, name+" "+target, rec.Body.String())
				h := rec.Header()
				if !strings.HasPrefix(h.Get("Content-Type"), "text/html") || h.Get("Cache-Control") != "private, no-store" {
					t.Errorf("%s %s: headers = %v", name, target, h)
				}
				if !hasFrameAncestorsNone(h) || h.Get("X-Frame-Options") != "DENY" {
					t.Errorf("%s %s: the gate page can be framed: %v", name, target, h)
				}
				if h.Get("ETag") != "" || h.Get("Last-Modified") != "" {
					t.Errorf("%s %s: validators leak file details: %v", name, target, h)
				}
				if h.Get("Referrer-Policy") != "no-referrer" {
					t.Errorf("%s %s: Referrer-Policy = %q", name, target, h.Get("Referrer-Policy"))
				}
			}
		})
	}
}

func TestGatePageContentMatchesTheMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		body := e.site("/").Body.String()
		for _, want := range []string{`type="password"`, UnlockPath, `name="token"`} {
			if !strings.Contains(body, want) {
				t.Errorf("password page lacks %q", want)
			}
		}
		if strings.Contains(body, "Sign in with your account") || strings.Contains(body, "<script") {
			t.Errorf("password page content: %s", body)
		}
		e.protect("demo", restrictedMode("sam@example.com"))
		body = e.site("/").Body.String()
		for _, want := range []string{"Sign in with your account", HandoffPath + "?return=%2F&amp;site=demo", `name="email"`, CodePath} {
			if !strings.Contains(body, want) {
				t.Errorf("restricted page lacks %q: %s", want, body)
			}
		}
		if strings.Contains(body, `type="password"`) {
			t.Error("restricted page has a password field")
		}
	})
}

func TestGatePagesLoadNothingExternalAndRunNoScript(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode("sam@example.com"))
		body := strings.ToLower(e.site("/").Body.String())
		for _, banned := range []string{"<script", "<link", "<img", "<iframe", "src=", "@import", "url(", "onclick", "onload"} {
			if strings.Contains(body, banned) {
				t.Errorf("gate page contains %q", banned)
			}
		}
		policy := e.site("/").Header().Get("Content-Security-Policy")
		for _, want := range []string{"default-src 'none'", "form-action 'self'", "frame-ancestors 'none'"} {
			if !strings.Contains(policy, want) {
				t.Errorf("policy %q lacks %q", policy, want)
			}
		}
	})
}

func TestGatePageEscapesTheRequestedPath(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		rec := e.site(`/%22%3E%3Cscript%3Ealert(1)%3C/script%3E?q=%22%3E%3Cb%3E`)
		body := rec.Body.String()
		if strings.Contains(body, "<script>alert") || strings.Contains(body, `"><b>`) {
			t.Errorf("the page reflects the path unescaped: %s", body)
		}
	})
}

func TestSubresourcesGetAPlainRefusal(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		cases := map[string][]string{
			"script":     {"Sec-Fetch-Dest", "script"},
			"style":      {"Sec-Fetch-Dest", "style"},
			"image":      {"Sec-Fetch-Dest", "image"},
			"fetch":      {"Sec-Fetch-Dest", "empty"},
			"json":       {"Accept", "application/json"},
			"image type": {"Accept", "image/avif,image/webp"},
		}
		for name, headers := range cases {
			rec := e.site("/_next/static/chunk-1.js", headers...)
			if rec.Code != http.StatusUnauthorized || rec.Body.String() != "Unauthorized\n" {
				t.Errorf("%s: %d %q", name, rec.Code, rec.Body.String())
			}
			if !hasFrameAncestorsNone(rec.Header()) || rec.Header().Get("Cache-Control") != "private, no-store" {
				t.Errorf("%s: headers = %v", name, rec.Header())
			}
		}
		for _, dest := range []string{"document", "iframe"} {
			if rec := e.site("/", "Sec-Fetch-Dest", dest); !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
				t.Errorf("%s navigation did not get the gate page", dest)
			}
		}
	})
}

func TestGatedHeadRequestsCarryNoBody(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		rec := e.request(http.MethodHead, siteHost, "/")
		if rec.Code != http.StatusUnauthorized || rec.Body.Len() != 0 {
			t.Errorf("HEAD = %d with %d body bytes", rec.Code, rec.Body.Len())
		}
	})
}

func TestConditionalAndRangeRequestsCannotProbeGatedFiles(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		for _, headers := range [][]string{
			{"If-None-Match", `"` + hashOf("home") + `"`},
			{"Range", "bytes=0-2"},
			{"If-Modified-Since", "Wed, 21 Oct 2037 07:28:00 GMT"},
		} {
			rec := e.site("/", headers...)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%v: status = %d", headers, rec.Code)
			}
		}
	})
}

func TestUnknownAndNotLiveSitesStayIndistinguishable(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		before := capture(e.get("nobody.sites.example.com", "/"))
		if before.status != http.StatusNotFound || before.body != "Not found\n" {
			t.Fatalf("unknown site = %+v", before)
		}
		if _, err := e.publish.Begin(t.Context(), beginDraft(e, "draft")); err != nil {
			t.Fatal(err)
		}
		e.protect("draft", passwordMode())
		if got := capture(e.get("draft.sites.example.com", "/")); got != before {
			t.Errorf("a gated site without a live version differs from an unknown one:\n%+v\n%+v", got, before)
		}
	})
}

func TestGatedPathsGetIdenticalResponsesWhetherOrNotTheyExist(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		strip := func(snap snapshot) snapshot {
			snap.body = tokenPattern.ReplaceAllString(snap.body, "")
			snap.body = returnField.ReplaceAllString(snap.body, "")
			return snap
		}
		real := strip(capture(e.site("/about")))
		for _, target := range []string{"/about", "/nothing-here", "/docs/", "/a/b/c/d"} {
			if got := strip(capture(e.site(target))); got != real {
				t.Errorf("%s differs from a real page:\n%+v\n%+v", target, got, real)
			}
		}
	})
}

func TestOnlyReadMethodsReachGatedContent(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		cookie := e.unlockedCookie(siteHost, "")
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			rec := e.request(method, siteHost, "/about", "Cookie", cookieHeader(cookie))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /about = %d", method, rec.Code)
			}
		}
		if rec := e.postForm(siteHost, "/.well-known/selfhost-gate/other", nil, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("POST to an unknown gate path = %d, want the gate page", rec.Code)
		}
	})
}

func TestGatePostsOnPublicSitesAreRefused(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.enableSubdomains()
		for _, target := range []string{UnlockPath, CodePath, VerifyPath} {
			if rec := e.postForm(siteHost, target, nil, ""); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("POST %s on a public site = %d", target, rec.Code)
			}
		}
		expectBody(t, e.site(CallbackPath+"?token=x"), 404, "custom missing")
	})
}
