package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/config"
	"github.com/zachlagden/openhere/internal/httptimeouts"
	"github.com/zachlagden/openhere/internal/settings"
)

type fakeDomains struct {
	fakeLookup
	names []string
}

func (f *fakeDomains) VerifiedNames(context.Context, int) ([]string, error) {
	return f.names, nil
}

type managerEnv struct {
	manager  *Manager
	settings *fakeSettings
	domains  *fakeDomains
	ca       *httptest.Server
	caHits   *atomic.Int64
	dir      string
}

func newManagerEnv(t *testing.T) *managerEnv {
	t.Helper()
	var hits atomic.Int64
	ca := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(ca.Close)

	cfg := settings.Defaults()
	cfg.BaseDomain = "sites.example.com"
	env := &managerEnv{
		settings: &fakeSettings{cfg: cfg},
		domains:  &fakeDomains{fakeLookup: fakeLookup{verified: map[string]bool{"www.example.org": true}}},
		ca:       ca,
		caHits:   &hits,
		dir:      filepath.Join(t.TempDir(), "certs"),
	}
	manager, err := New(Options{
		TLS: config.TLS{
			Enabled: true, HTTPAddr: ":80", HTTPSAddr: ":443", Email: "admin@example.com", CA: ca.URL, Storage: env.dir,
		},
		Public:   config.PublicURL{Scheme: "https", Host: "admin.example.com"},
		Settings: env.settings, Secrets: testBox(t), Domains: env.domains,
		ReconcileInterval: time.Hour,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)
	env.manager = manager
	return env
}

func selfSigned(t *testing.T, notAfter time.Time, names ...string) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: names[0]}, DNSNames: names,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func (e *managerEnv) cacheCert(t *testing.T, notAfter time.Time, names ...string) {
	t.Helper()
	certPEM, keyPEM := selfSigned(t, notAfter, names...)
	if _, err := e.manager.static.CacheUnmanagedCertificatePEMBytes(context.Background(), certPEM, keyPEM, nil); err != nil {
		t.Fatal(err)
	}
}

func handshake(t *testing.T, config *tls.Config, serverName string) error {
	t.Helper()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deadline := time.Now().Add(5 * time.Second)
	if err := client.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := server.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	serverConn := tls.Server(server, config)
	done := make(chan error, 1)
	go func() {
		err := serverConn.HandshakeContext(ctx)
		if err != nil {
			_ = server.Close()
		}
		done <- err
	}()
	clientConn := tls.Client(client, &tls.Config{ServerName: serverName, InsecureSkipVerify: true})
	clientErr := clientConn.HandshakeContext(ctx)
	serverErr := <-done
	if serverErr != nil {
		return serverErr
	}
	return clientErr
}

func TestStorageIsPrivate(t *testing.T) {
	env := newManagerEnv(t)
	info, err := os.Stat(env.dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("storage = %v, %v", info, err)
	}
}

func TestNewRejectsAnUnreadableCARoot(t *testing.T) {
	_, err := New(Options{TLS: config.TLS{Storage: t.TempDir(), CARoot: filepath.Join(t.TempDir(), "missing.pem")}})
	if err == nil {
		t.Error("a missing CA root was accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{TLS: config.TLS{Storage: t.TempDir(), CARoot: bad}}); err == nil {
		t.Error("a CA root without a certificate was accepted")
	}
}

func TestUnknownServerNamesGetNoCertificateAndNoACMETraffic(t *testing.T) {
	env := newManagerEnv(t)
	for _, name := range []string{
		"unknown.example.net", "pending.example.org", "launch.sites.example.com", "sites.example.com", "", "1.2.3.4",
	} {
		if err := handshake(t, env.manager.TLSConfig(), name); err == nil {
			t.Errorf("a handshake for %q succeeded", name)
		}
	}
	if hits := env.caHits.Load(); hits != 0 {
		t.Errorf("the CA received %d requests for refused names", hits)
	}
}

func TestVerifiedCustomDomainsReachTheCA(t *testing.T) {
	env := newManagerEnv(t)
	if err := handshake(t, env.manager.TLSConfig(), "www.example.org"); err == nil {
		t.Error("a handshake succeeded against a CA that always fails")
	}
	if env.caHits.Load() == 0 {
		t.Error("an allowed name never reached the CA")
	}
}

func TestCachedCertificatesAreServed(t *testing.T) {
	env := newManagerEnv(t)
	env.cacheCert(t, time.Now().Add(48*time.Hour), "*.sites.example.com")
	if err := handshake(t, env.manager.TLSConfig(), "launch.sites.example.com"); err != nil {
		t.Errorf("a subdomain with a wildcard certificate: %v", err)
	}
	if err := handshake(t, env.manager.TLSConfig(), "deep.launch.sites.example.com"); err == nil {
		t.Error("a wildcard certificate served two labels deep")
	}
	if env.caHits.Load() != 0 {
		t.Errorf("CA hits = %d", env.caHits.Load())
	}
}

func TestTLSConfigIsModern(t *testing.T) {
	env := newManagerEnv(t)
	config := env.manager.TLSConfig()
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("min version = %x", config.MinVersion)
	}
	if !contains(config.NextProtos, "h2") || !contains(config.NextProtos, "http/1.1") {
		t.Errorf("protocols = %v", config.NextProtos)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestServersHaveTimeoutsAndQuietErrorLogs(t *testing.T) {
	env := newManagerEnv(t)
	plain := env.manager.HTTPServer(":80")
	secure := env.manager.HTTPSServer(":443", http.NotFoundHandler(), httptimeouts.Defaults())
	for name, srv := range map[string]*http.Server{"http": plain, "https": secure} {
		if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
			t.Errorf("%s server lacks timeouts", name)
		}
		if srv.ErrorLog == nil {
			t.Errorf("%s server logs handshake errors", name)
		}
	}
	if secure.TLSConfig == nil || plain.TLSConfig != nil {
		t.Error("only the HTTPS server should carry a TLS config")
	}
}

func TestStatusReportsCertificatesAndDNS(t *testing.T) {
	env := newManagerEnv(t)
	ctx := context.Background()
	status, err := env.manager.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.ManagementExpiresAt != nil || status.Wildcard.State != WildcardNone ||
		status.Custom.Count != 0 || status.DNSProviderConfigured {
		t.Errorf("empty status = %+v", status)
	}

	managementExpiry := time.Now().Add(60 * 24 * time.Hour)
	soonest := time.Now().Add(10 * 24 * time.Hour)
	env.cacheCert(t, managementExpiry, "admin.example.com")
	env.cacheCert(t, soonest, "www.example.org")
	env.cacheCert(t, time.Now().Add(30*24*time.Hour), "shop.example.org")
	env.cacheCert(t, time.Now().Add(80*24*time.Hour), "*.sites.example.com")
	env.domains.names = []string{"www.example.org", "shop.example.org", "missing.example.org"}
	env.settings.cfg = sealedSettings(t, testBox(t), settings.DNSProviderCloudflare, "example-token")
	env.settings.cfg.BaseDomain = "sites.example.com"

	status, err = env.manager.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.ManagementExpiresAt == nil || status.ManagementExpiresAt.Sub(managementExpiry.UTC()).Abs() > 2*time.Second {
		t.Errorf("management expiry = %v", status.ManagementExpiresAt)
	}
	if status.Custom.Count != 2 || status.Custom.SoonestExpiry == nil || status.Custom.SoonestExpiry.Sub(soonest.UTC()).Abs() > 2*time.Second {
		t.Errorf("custom = %+v", status.Custom)
	}
	if status.Wildcard.State != WildcardOK || !status.DNSProviderConfigured {
		t.Errorf("wildcard = %+v, dns = %v", status.Wildcard, status.DNSProviderConfigured)
	}
}

func TestExpiredCertificatesDoNotCount(t *testing.T) {
	env := newManagerEnv(t)
	env.cacheCert(t, time.Now().Add(-time.Minute), "admin.example.com")
	status, err := env.manager.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.ManagementExpiresAt != nil {
		t.Errorf("expired certificate reported: %v", status.ManagementExpiresAt)
	}
}

func TestWildcardStateFollowsIssuanceEvents(t *testing.T) {
	env := newManagerEnv(t)
	manager := env.manager
	name := "*.sites.example.com"
	state := func() WildcardStatus {
		_, status := manager.wild.snapshot()
		return status
	}
	manager.wild.begin(name)
	if state().State != WildcardPending {
		t.Fatalf("state = %+v", state())
	}
	_ = manager.onWildcardEvent(context.Background(), "cert_failed", map[string]any{"identifier": name, "error": "dns provider said no"})
	if got := state(); got.State != WildcardFailed || got.Reason == "" || strings.Contains(got.Reason, "dns provider said no") {
		t.Errorf("failed state = %+v", got)
	}
	_ = manager.onWildcardEvent(context.Background(), "cert_obtaining", map[string]any{"identifier": name})
	if state().State != WildcardFailed {
		t.Errorf("a retry hid the failure: %+v", state())
	}
	_ = manager.onWildcardEvent(context.Background(), "cert_obtained", map[string]any{"identifier": name})
	if state().State != WildcardOK {
		t.Errorf("state = %+v", state())
	}
	_ = manager.onWildcardEvent(context.Background(), "cert_failed", map[string]any{"identifier": "*.other.example.com"})
	if state().State != WildcardOK {
		t.Errorf("an event for another name changed the state: %+v", state())
	}
}

func TestReconcileStartsAndStopsTheWildcard(t *testing.T) {
	env := newManagerEnv(t)
	ctx := t.Context()
	if err := env.manager.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if name, _ := env.manager.wild.snapshot(); name != "" {
		t.Errorf("wildcard managed without a provider: %q", name)
	}

	env.settings.cfg = sealedSettings(t, testBox(t), settings.DNSProviderCloudflare, "example-token")
	env.settings.cfg.BaseDomain = "sites.example.com"
	if err := env.manager.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	name, status := env.manager.wild.snapshot()
	if name != "*.sites.example.com" || status.State == WildcardNone {
		t.Errorf("tracker = %q, %+v", name, status)
	}

	env.settings.cfg.TLS.DNSProvider = settings.DNSProviderNone
	if err := env.manager.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if name, status := env.manager.wild.snapshot(); name != "" || status.State != WildcardNone {
		t.Errorf("tracker after removing the provider = %q, %+v", name, status)
	}
}

func TestRedirectSendsEveryRequestToHTTPS(t *testing.T) {
	env := newManagerEnv(t)
	handler := env.manager.HTTPServer(":80").Handler
	cases := []struct {
		method, target, host, want string
	}{
		{"GET", "/", "admin.example.com", "https://admin.example.com/"},
		{"GET", "/dashboard/sites?page=2", "Admin.Example.com:80", "https://admin.example.com/dashboard/sites?page=2"},
		{"POST", "/api/v1/auth/login", "admin.example.com", "https://admin.example.com/api/v1/auth/login"},
		{"GET", "/a%2Fb/c", "launch.sites.example.com", "https://launch.sites.example.com/a%2Fb/c"},
		{"GET", "/s/launch/", "[2001:db8::1]:80", "https://[2001:db8::1]/s/launch/"},
		{"GET", "/.well-known/acme-challenge/unknown-token", "admin.example.com", "https://admin.example.com/.well-known/acme-challenge/unknown-token"},
		{"GET", "/api/v1/setup/status", "www.example.org", "https://www.example.org/api/v1/setup/status"},
	}
	for _, tc := range cases {
		req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, nil)
		req.Host = tc.host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != tc.want {
			t.Errorf("%s %s (%s) = %d %q, want 301 %q", tc.method, tc.target, tc.host, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}

func TestRedirectKeepsANonStandardHTTPSPort(t *testing.T) {
	handler := redirectHandler("8443")
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/x?y=1", nil)
	req.Host = "admin.example.com:8080"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Location"); got != "https://admin.example.com:8443/x?y=1" {
		t.Errorf("Location = %q", got)
	}
}

func TestRedirectRejectsOddHosts(t *testing.T) {
	handler := redirectHandler("443")
	for _, host := range []string{"", "evil.example.com/path", "a b.example.com", "user@evil.example.com", "evil.example.com\\x", "[not-an-ip]"} {
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("host %q = %d, want 400", host, rec.Code)
		}
	}
}

func TestPlainHTTPServerNeverServesContent(t *testing.T) {
	env := newManagerEnv(t)
	handler := env.manager.HTTPServer(":80").Handler
	for _, target := range []string{"/", "/api/v1/me", "/health", "/.well-known/openhere-check/abc", "/.well-known/openhere-auth", "/s/launch/index.html"} {
		req := httptest.NewRequestWithContext(t.Context(), "GET", target, nil)
		req.Host = "admin.example.com"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently || rec.Body.Len() > 256 {
			t.Errorf("GET %s = %d with %d body bytes", target, rec.Code, rec.Body.Len())
		}
	}
}
