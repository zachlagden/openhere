package hosting

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

type fakeResolver struct {
	mu    sync.Mutex
	hosts []string
	addrs []string
	err   error
}

func (f *fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hosts = append(f.hosts, host)
	return f.addrs, f.err
}

type doerFunc func(req *http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func routeToApp(e *env, requests *[]*http.Request) doerFunc {
	return func(req *http.Request) (*http.Response, error) {
		*requests = append(*requests, req)
		rec := httptest.NewRecorder()
		forwarded := req.Clone(req.Context())
		forwarded.Host = req.URL.Host
		e.router.ServeHTTP(rec, forwarded)
		return rec.Result(), nil
	}
}

func newChecker(e *env, resolver HostResolver, client Doer, public config.PublicURL) *Checker {
	return NewChecker(e.settings, e.probes, CheckerOptions{
		Public: public, Resolver: resolver, Client: client, Timeout: 100 * time.Millisecond,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Sleep:  func(context.Context, time.Duration) error { return nil },
	})
}

var httpsPublic = config.PublicURL{Scheme: "https", Host: managementHost}

func TestWildcardCheckSucceedsWhenTheAppAnswersItself(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		var requests []*http.Request
		resolver := &fakeResolver{addrs: []string{"192.0.2.10"}}
		checker := newChecker(e, resolver, routeToApp(e, &requests), config.PublicURL{Scheme: "https", Host: managementHost, Port: "8443"})

		result, err := checker.Check(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if result.WildcardStatus != settings.WildcardOK || result.WildcardDetail != "" || result.WildcardCheckedAt == "" {
			t.Errorf("result = %+v", result)
		}
		if len(requests) != 1 || len(resolver.hosts) != 1 {
			t.Fatalf("requests = %d, lookups = %d", len(requests), len(resolver.hosts))
		}
		req := requests[0]
		label := strings.TrimSuffix(resolver.hosts[0], "."+managementHost)
		if !strings.HasPrefix(label, "check-") || strings.Contains(label, ".") {
			t.Errorf("probe host = %q", resolver.hosts[0])
		}
		if req.URL.Scheme != "https" || req.URL.Host != resolver.hosts[0]+":8443" || !strings.HasPrefix(req.URL.Path, ProbePath) || req.Method != http.MethodGet {
			t.Errorf("probe request = %s %s", req.Method, req.URL)
		}
		if len(e.probes.tokens) != 0 {
			t.Error("the probe token was not cleaned up")
		}
	})
}

func TestWildcardCheckUsesTheHttpSchemeOfPublicURL(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		var seen string
		client := doerFunc(func(req *http.Request) (*http.Response, error) {
			seen = req.URL.String()
			return reply(404, ""), nil
		})
		checker := newChecker(e, &fakeResolver{addrs: []string{"192.0.2.10"}}, client, config.PublicURL{Scheme: "http", Host: managementHost})
		if _, err := checker.Check(t.Context()); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(seen, "http://check-") {
			t.Errorf("probe URL = %q", seen)
		}
	})
}

func TestWildcardCheckFailures(t *testing.T) {
	notOurs := doerFunc(func(*http.Request) (*http.Response, error) { return reply(200, "welcome to another server"), nil })
	cases := []struct {
		name     string
		resolver *fakeResolver
		client   Doer
		want     string
	}{
		{"DNS error", &fakeResolver{err: errors.New("no such host")}, notOurs, "DNS did not resolve"},
		{"DNS with no addresses", &fakeResolver{}, notOurs, "DNS did not resolve"},
		{"wrong body", &fakeResolver{addrs: []string{"192.0.2.10"}}, notOurs, "not with this server's check token"},
		{"impostor echoing the token", &fakeResolver{addrs: []string{"192.0.2.10"}}, doerFunc(func(req *http.Request) (*http.Response, error) {
			return reply(200, strings.TrimPrefix(req.URL.Path, ProbePath)), nil
		}), "not with this server's check token"},
		{"wrong status", &fakeResolver{addrs: []string{"192.0.2.10"}}, doerFunc(func(*http.Request) (*http.Response, error) { return reply(503, ""), nil }), "HTTP 503"},
		{"redirect", &fakeResolver{addrs: []string{"192.0.2.10"}}, doerFunc(func(*http.Request) (*http.Response, error) { return reply(301, ""), nil }), "redirect"},
		{"connection error", &fakeResolver{addrs: []string{"192.0.2.10"}}, doerFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") }), "connection refused"},
		{"timeout", &fakeResolver{addrs: []string{"192.0.2.10"}}, doerFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}), "context deadline exceeded"},
	}
	for _, tc := range cases {
		storetest.Each(t, func(t *testing.T, db store.Store) {
			e := newEnv(t, db)
			e.enableSubdomains()
			checker := newChecker(e, tc.resolver, tc.client, httpsPublic)

			start := time.Now()
			result, err := checker.Check(t.Context())
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if time.Since(start) > 5*time.Second {
				t.Errorf("%s: the check took %s", tc.name, time.Since(start))
			}
			if result.WildcardStatus != settings.WildcardFailed || !strings.Contains(result.WildcardDetail, tc.want) {
				t.Errorf("%s: status %q, detail %q, want failed mentioning %q", tc.name, result.WildcardStatus, result.WildcardDetail, tc.want)
			}
			if result.SubdomainEnabled {
				t.Errorf("%s: a failed check left subdomain mode on", tc.name)
			}
			if len(e.probes.tokens) != 0 {
				t.Errorf("%s: the probe token was not revoked", tc.name)
			}
		})
	}
}

func TestWildcardCheckNeedsABaseDomain(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.settings = settings.New(db, settings.Options{})
		checker := newChecker(e, &fakeResolver{addrs: []string{"192.0.2.10"}}, routeToApp(e, new([]*http.Request)), httpsPublic)
		result, err := checker.Check(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if result.WildcardStatus != settings.WildcardFailed || !strings.Contains(result.WildcardDetail, "No base domain") {
			t.Errorf("result = %+v", result)
		}
	})
}

func TestWildcardCheckCapsTheResponseSize(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		counter := &countingReader{}
		client := doerFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(counter), Header: http.Header{}}, nil
		})
		checker := newChecker(e, &fakeResolver{addrs: []string{"192.0.2.10"}}, client, httpsPublic)
		result, err := checker.Check(t.Context())
		if err != nil || result.WildcardStatus != settings.WildcardFailed {
			t.Fatalf("result = %+v, %v", result, err)
		}
		if counter.read > maxProbeBody*2 {
			t.Errorf("the check read %d bytes of an endless body", counter.read)
		}
	})
}

type countingReader struct{ read int }

func (c *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	c.read += len(p)
	return len(p), nil
}

func TestProbeClientNeverFollowsRedirects(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		resp := reply(http.StatusFound, "")
		resp.Header.Set("Location", "https://elsewhere.example.org/")
		resp.Request = req
		return resp, nil
	})
	client := newProbeClient(time.Second, transport)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://check-1.sites.example.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || calls != 1 {
		t.Errorf("status = %d after %d calls, want the first redirect returned untouched", resp.StatusCode, calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestProbeRouteWorksOnceAndOnlyOnSiteHosts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		path := func(token string) string { return ProbePath + token }

		generic := capture(e.get("nobody.sites.example.com", "/anything"))
		if got := capture(e.get("check-1.sites.example.com", path("guess"))); got != generic {
			t.Errorf("a probe path without a token differs from the generic 404:\n%+v\n%+v", got, generic)
		}

		token := e.probes.Issue()
		before := e.managed.Load()
		onManagement := e.get(managementHost, path(token))
		if onManagement.Body.String() != "management" || e.managed.Load() != before+1 {
			t.Errorf("the management host answered the probe: %d %q", onManagement.Code, onManagement.Body.String())
		}
		wrong := e.get("check-1.sites.example.com", path("0123456789abcdef"))
		if capture(wrong) != generic {
			t.Errorf("a wrong token was not a generic 404: %+v", capture(wrong))
		}
		post := e.request("POST", "check-1.sites.example.com", path(token))
		if post.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST = %d", post.Code)
		}

		ok := e.get("check-1.sites.example.com", path(token))
		if ok.Code != 200 || ok.Body.String() != e.probes.Proof(token) || ok.Body.String() == token || ok.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("probe = %d %q", ok.Code, ok.Body.String())
		}
		again := e.get("check-1.sites.example.com", path(token))
		if capture(again) != generic {
			t.Errorf("a token worked twice: %+v", capture(again))
		}
	})
}

func TestProbeTokensExpireAfterSixtySeconds(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	probes := NewProbes(clock.Now)
	fresh := probes.Issue()
	late := probes.Issue()
	clock.now = clock.now.Add(59 * time.Second)
	if !consumed(probes, fresh) {
		t.Error("a token at 59 seconds was refused")
	}
	clock.now = clock.now.Add(2 * time.Second)
	if consumed(probes, late) {
		t.Error("a token at 61 seconds was accepted")
	}
	if consumed(probes, late) || consumed(probes, "") || consumed(probes, "unknown") {
		t.Error("an unknown or spent token was accepted")
	}
	if len(probes.tokens) != 0 {
		t.Errorf("%d tokens remain", len(probes.tokens))
	}
	other := probes.Issue()
	if other == fresh || len(other) < 26 {
		t.Errorf("token = %q", other)
	}
	probes.Revoke(other)
	if consumed(probes, other) {
		t.Error("a revoked token was accepted")
	}
}

func TestScheduledCheckRetriesBeforeFailing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.enableSubdomains()
		attempts := 0
		client := doerFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return reply(502, ""), nil
			}
			rec := httptest.NewRecorder()
			forwarded := req.Clone(req.Context())
			forwarded.Host = req.URL.Host
			e.router.ServeHTTP(rec, forwarded)
			return rec.Result(), nil
		})
		checker := newChecker(e, &fakeResolver{addrs: []string{"192.0.2.10"}}, client, httpsPublic)
		checker.runScheduled(t.Context())

		got, err := e.settings.Get(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if attempts != 3 || got.WildcardStatus != settings.WildcardOK || !got.SubdomainEnabled {
			t.Errorf("attempts = %d, settings = %+v", attempts, got)
		}

		attempts = -10
		checker.runScheduled(t.Context())
		got, err = e.settings.Get(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if got.WildcardStatus != settings.WildcardFailed || got.SubdomainEnabled {
			t.Errorf("after three failures: %+v", got)
		}
	})
}

func TestRunStopsWhenTheContextEnds(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		checker := NewChecker(e.settings, e.probes, CheckerOptions{Public: httpsPublic, Resolver: &fakeResolver{}})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		done := make(chan struct{})
		go func() {
			checker.Run(ctx, time.Hour)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Run ignored the cancelled context")
		}
	})
}

func consumed(probes *Probes, token string) bool {
	_, ok := probes.Consume(token)
	return ok
}
