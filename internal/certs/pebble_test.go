package certs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/libdns"
	"github.com/miekg/dns"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/settings"
)

type challengeDNS struct {
	mu   sync.Mutex
	txt  map[string][]string
	addr string
}

func startChallengeDNS(t *testing.T, addr string) *challengeDNS {
	t.Helper()
	server := &challengeDNS{txt: map[string][]string{}, addr: addr}
	for _, network := range []string{"udp", "tcp"} {
		started := make(chan struct{})
		dnsServer := &dns.Server{Addr: addr, Net: network, Handler: server, NotifyStartedFunc: func() { close(started) }}
		go func() { _ = dnsServer.ListenAndServe() }()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("the %s DNS server did not start on %s", network, addr)
		}
		t.Cleanup(func() { _ = dnsServer.Shutdown() })
	}
	return server
}

func (c *challengeDNS) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	reply := new(dns.Msg)
	reply.SetReply(req)
	reply.Authoritative = true
	for _, question := range req.Question {
		header := dns.RR_Header{Name: question.Name, Rrtype: question.Qtype, Class: dns.ClassINET, Ttl: 1}
		switch question.Qtype {
		case dns.TypeA:
			reply.Answer = append(reply.Answer, &dns.A{Hdr: header, A: net.ParseIP("127.0.0.1")})
		case dns.TypeSOA:
			reply.Answer = append(reply.Answer, &dns.SOA{Hdr: header, Ns: "ns.example.net.", Mbox: "hostmaster.example.net.", Serial: 1, Minttl: 1})
		case dns.TypeTXT:
			for _, value := range c.values(question.Name) {
				reply.Answer = append(reply.Answer, &dns.TXT{Hdr: header, Txt: []string{value}})
			}
		}
	}
	_ = w.WriteMsg(reply)
}

func (c *challengeDNS) values(name string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.txt[strings.ToLower(name)])
}

func recordHost(name, zone string) string {
	if name == "@" || name == "" {
		return strings.ToLower(zone)
	}
	return strings.ToLower(name + "." + strings.TrimSuffix(zone, ".") + ".")
}

func (c *challengeDNS) AppendRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, record := range records {
		rr := record.RR()
		host := recordHost(rr.Name, zone)
		c.txt[host] = append(c.txt[host], rr.Data)
	}
	return records, nil
}

func (c *challengeDNS) DeleteRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, record := range records {
		delete(c.txt, recordHost(record.RR().Name, zone))
	}
	return records, nil
}

type pebbleEnv struct {
	manager *Manager
}

func newPebbleEnv(t *testing.T) *pebbleEnv {
	t.Helper()
	directory := os.Getenv("PEBBLE_DIRECTORY")
	rootFile := os.Getenv("PEBBLE_CA_ROOT")
	resolver := os.Getenv("PEBBLE_DNS")
	if directory == "" || rootFile == "" || resolver == "" {
		t.Skip("set PEBBLE_DIRECTORY, PEBBLE_CA_ROOT and PEBBLE_DNS to run the ACME test against Pebble, which must use PEBBLE_DNS as its DNS server")
	}
	box := testBox(t)
	cfg := sealedSettings(t, box, settings.DNSProviderCloudflare, "example-token")
	cfg.BaseDomain = "sites.example.com"
	provider := startChallengeDNS(t, resolver)
	manager, err := New(Options{
		TLS: config.TLS{
			Enabled: true, HTTPAddr: ":5002", HTTPSAddr: ":5001", Email: "admin@example.com",
			CA: directory, CARoot: rootFile, Storage: filepath.Join(t.TempDir(), "certs"),
		},
		Public:   config.PublicURL{Scheme: "https", Host: "admin.example.com", Port: "5001"},
		Settings: &fakeSettings{cfg: cfg}, Secrets: box,
		Domains: &fakeDomains{fakeLookup: fakeLookup{verified: map[string]bool{"www.acme-shop.org": true}}},
		Providers: map[string]ProviderBuilder{
			settings.DNSProviderCloudflare: func(string) certmagic.DNSProvider { return provider },
		},
		ReconcileInterval: time.Hour,
		Logger:            slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)
	tuneDNS(t, manager, resolver)
	return &pebbleEnv{manager: manager}
}

func tuneDNS(t *testing.T, manager *Manager, resolver string) {
	t.Helper()
	issuer, ok := manager.wildcard.Issuers[0].(*certmagic.ACMEIssuer)
	if !ok {
		t.Fatal("the wildcard issuer is not an ACME issuer")
	}
	solver, ok := issuer.DNS01Solver.(*certmagic.DNS01Solver)
	if !ok {
		t.Fatal("the wildcard issuer has no DNS solver")
	}
	solver.PropagationDelay = time.Second
	solver.PropagationTimeout = -1
	solver.Resolvers = []string{resolver}
}

func (e *pebbleEnv) serve(t *testing.T) {
	t.Helper()
	plain := e.manager.HTTPServer(":5002")
	secure := e.manager.HTTPSServer(":5001", http.NotFoundHandler())
	go func() { _ = plain.ListenAndServe() }()
	go func() { _ = secure.ListenAndServeTLS("", "") }()
	t.Cleanup(func() {
		_ = plain.Close()
		_ = secure.Close()
	})
	if err := e.manager.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func fetchCertificate(t *testing.T, name string) (*x509.Certificate, error) {
	t.Helper()
	dialer := &tls.Dialer{Config: &tls.Config{ServerName: name, InsecureSkipVerify: true}}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:5001")
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok || len(tlsConn.ConnectionState().PeerCertificates) == 0 {
		return nil, errors.New("no certificate")
	}
	return tlsConn.ConnectionState().PeerCertificates[0], nil
}

func TestPebbleIssuesManagementWildcardAndCustomDomainCertificates(t *testing.T) {
	env := newPebbleEnv(t)
	env.serve(t)

	deadline := time.Now().Add(90 * time.Second)
	for {
		status, err := env.manager.Status(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if status.Wildcard.State == WildcardOK && status.ManagementExpiresAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("certificates were not issued in time: %+v", status)
		}
		time.Sleep(time.Second)
	}

	wildcard, err := fetchCertificate(t, "launch.sites.example.com")
	if err != nil || !slices.Contains(wildcard.DNSNames, "*.sites.example.com") {
		t.Fatalf("subdomain certificate = %v, %v", wildcard, err)
	}
	custom, err := fetchCertificate(t, "www.acme-shop.org")
	if err != nil || !slices.Contains(custom.DNSNames, "www.acme-shop.org") {
		t.Fatalf("custom domain certificate = %v, %v", custom, err)
	}
	if _, err := fetchCertificate(t, "unverified.acme-shop.org"); err == nil {
		t.Error("an unverified name received a certificate")
	}
}
