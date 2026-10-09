package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	testPassword = "correct horse battery"
	testHost     = "example.com"
)

var testParams = auth.Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type harness struct {
	t       *testing.T
	handler http.Handler
	clock   *fakeClock
	auth    *auth.Service
	gate    *auth.SetupGate
	publish *publish.Service
	dataDir string
}

type harnessOptions struct {
	configured bool
	limits     *Limits
	proxies    []netip.Prefix
	public     config.PublicURL
}

func newHarness(t *testing.T, db store.Store, opts harnessOptions) *harness {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.New(db, auth.Options{KeyPrefix: "sk_", Params: testParams, Now: clock.Now, Logger: logger})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	gate := auth.ClosedSetupGate()
	if !opts.configured {
		gate, err = auth.NewSetupGate()
		if err != nil {
			t.Fatalf("NewSetupGate: %v", err)
		}
	}
	limits := Limits{
		LoginPerIP:    Rule{Max: 1000, Window: time.Hour},
		LoginPerEmail: Rule{Max: 1000, Window: time.Hour},
		SetupPerIP:    Rule{Max: 1000, Window: time.Hour},
		SetupPerEmail: Rule{Max: 1000, Window: time.Hour},
	}
	if opts.limits != nil {
		limits = *opts.limits
	}
	dataDir := t.TempDir()
	files, err := sitefiles.New(dataDir)
	if err != nil {
		t.Fatalf("sitefiles.New: %v", err)
	}
	workspaceSvc := workspaces.New(db, workspaces.Options{Now: clock.Now})
	siteSvc := sites.New(db, workspaceSvc, sites.Options{Now: clock.Now, Files: files, Logger: logger})
	settingsSvc := settings.New(db, settings.Options{Now: clock.Now})
	publishSvc := publish.New(db, siteSvc, workspaceSvc, settingsSvc, files, publish.Options{Now: clock.Now, Logger: logger})
	server := New(svc, workspaceSvc, siteSvc, publishSvc, settingsSvc, gate, Options{TrustedProxies: opts.proxies, Limits: limits, Now: clock.Now, Logger: logger, Public: opts.public})
	return &harness{t: t, handler: server.Handler(), clock: clock, auth: svc, gate: gate, publish: publishSvc, dataDir: dataDir}
}

type requestOption func(*http.Request)

func browser() requestOption {
	return func(r *http.Request) {
		r.Header.Set("Origin", "http://"+testHost)
		r.Header.Set("X-Requested-With", "fetch")
	}
}

func withCookie(c *http.Cookie) requestOption {
	return func(r *http.Request) {
		if c != nil {
			r.Header.Add("Cookie", c.Name+"="+c.Value)
		}
	}
}

func withBearer(key string) requestOption {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+key) }
}

func withHeader(name, value string) requestOption {
	return func(r *http.Request) { r.Header.Set(name, value) }
}

func fromAddr(addr string) requestOption {
	return func(r *http.Request) { r.RemoteAddr = addr }
}

func (h *harness) do(method, path string, body any, opts ...requestOption) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			reader = strings.NewReader(v)
		default:
			encoded, err := json.Marshal(body)
			if err != nil {
				h.t.Fatalf("encode body: %v", err)
			}
			reader = bytes.NewReader(encoded)
		}
	}
	req := httptest.NewRequestWithContext(h.t.Context(), method, "http://"+testHost+path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range opts {
		opt(req)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName || c.Name == secureSessionCookieName {
			return c
		}
	}
	return nil
}

type errorResponse struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RetryAfter *int   `json:"retry_after"`
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return out
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body.String())
	}
}

func expectError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) errorResponse {
	t.Helper()
	expectStatus(t, rec, status)
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := decodeBody[errorResponse](t, rec)
	if body.Code != code {
		t.Errorf("error code = %q, want %q", body.Code, code)
	}
	if body.Message == "" {
		t.Error("error message is empty")
	}
	return body
}

type setupResult struct {
	cookie *http.Cookie
	user   userResponse
}

func (h *harness) completeSetup() setupResult {
	h.t.Helper()
	rec := h.do("POST", "/api/v1/setup", setupRequest{Email: "alex@example.com", Password: testPassword, SetupToken: h.gate.Token()})
	expectStatus(h.t, rec, http.StatusCreated)
	cookie := sessionCookie(rec)
	if cookie == nil {
		h.t.Fatal("setup did not set a session cookie")
	}
	return setupResult{cookie: cookie, user: decodeBody[userEnvelope](h.t, rec).User}
}

func (h *harness) createKey(cookie *http.Cookie, label string) createKeyResponse {
	h.t.Helper()
	rec := h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: label}, browser(), withCookie(cookie))
	expectStatus(h.t, rec, http.StatusCreated)
	return decodeBody[createKeyResponse](h.t, rec)
}
