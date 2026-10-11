package certs

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"

	"github.com/zachlagden/openhere/internal/config"
)

const (
	storageMode       = 0o700
	reconcileInterval = 20 * time.Second
	wildcardPrefix    = "*."
)

type DomainSource interface {
	DomainLookup
	VerifiedNames(ctx context.Context, limit int) ([]string, error)
}

type Options struct {
	TLS               config.TLS
	Public            config.PublicURL
	Settings          SettingsReader
	Secrets           SecretOpener
	Domains           DomainSource
	Providers         map[string]ProviderBuilder
	Limits            Limits
	Logger            *slog.Logger
	Now               func() time.Time
	ReconcileInterval time.Duration
}

type Manager struct {
	settings  SettingsReader
	domains   DomainSource
	logger    *slog.Logger
	now       func() time.Time
	host      string
	httpsPort string
	interval  time.Duration

	cache      *certmagic.Cache
	onDemand   *certmagic.Config
	static     *certmagic.Config
	wildcard   *certmagic.Config
	httpIssuer *certmagic.ACMEIssuer
	policy     *Policy
	live       *liveProvider
	wild       *wildcardTracker

	mu          sync.Mutex
	managed     string
	fingerprint string
	cancelWild  context.CancelFunc
	loadedBase  string
}

func New(opts Options) (*Manager, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	interval := opts.ReconcileInterval
	if interval <= 0 {
		interval = reconcileInterval
	}
	if err := prepareStorage(opts.TLS.Storage); err != nil {
		return nil, err
	}
	roots, err := loadRoots(opts.TLS.CARoot)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		settings: opts.Settings, domains: opts.Domains, logger: logger, now: now,
		host: normaliseName(opts.Public.Host), httpsPort: opts.TLS.HTTPSPort(), interval: interval,
		wild: &wildcardTracker{state: WildcardNone},
		live: newLiveProvider(opts.Settings, opts.Secrets, opts.Providers),
		policy: NewPolicy(opts.Settings, opts.Domains, PolicyOptions{
			ManagementHost: opts.Public.Host, Limits: opts.Limits, Now: now, Logger: logger,
		}),
	}
	m.build(opts.TLS, roots)
	return m, nil
}

func (m *Manager) build(tlsConfig config.TLS, roots *x509.CertPool) {
	zapLogger := newCertmagicLogger(m.logger)
	m.cache = certmagic.NewCache(certmagic.CacheOptions{GetConfigForCert: m.configFor, Logger: zapLogger})
	template := certmagic.Config{
		Storage: &certmagic.FileStorage{Path: tlsConfig.Storage},
		Logger:  zapLogger,
	}
	issuer := certmagic.ACMEIssuer{
		Logger: zapLogger, CA: tlsConfig.CA, Email: tlsConfig.Email, Agreed: true, TrustedRoots: roots,
		AltHTTPPort: portNumber(tlsConfig.HTTPPort()), AltTLSALPNPort: portNumber(tlsConfig.HTTPSPort()),
	}

	onDemandTemplate := template
	onDemandTemplate.OnDemand = &certmagic.OnDemandConfig{DecisionFunc: m.policy.Allow}
	m.onDemand = certmagic.New(m.cache, onDemandTemplate)
	m.httpIssuer = certmagic.NewACMEIssuer(m.onDemand, issuer)
	m.onDemand.Issuers = []certmagic.Issuer{limitedIssuer{inner: m.httpIssuer, policy: m.policy}}

	m.static = certmagic.New(m.cache, template)
	m.static.Issuers = []certmagic.Issuer{certmagic.NewACMEIssuer(m.static, issuer)}

	wildcardTemplate := template
	wildcardTemplate.OnEvent = m.onWildcardEvent
	m.wildcard = certmagic.New(m.cache, wildcardTemplate)
	dnsIssuer := issuer
	dnsIssuer.DisableHTTPChallenge = true
	dnsIssuer.DisableTLSALPNChallenge = true
	dnsIssuer.DNS01Solver = &certmagic.DNS01Solver{DNSManager: certmagic.DNSManager{DNSProvider: m.live}}
	m.wildcard.Issuers = []certmagic.Issuer{certmagic.NewACMEIssuer(m.wildcard, dnsIssuer)}
}

func (m *Manager) configFor(cert certmagic.Certificate) (*certmagic.Config, error) {
	for _, name := range cert.Names {
		switch {
		case strings.HasPrefix(name, wildcardPrefix):
			return m.wildcard, nil
		case name == m.host:
			return m.static, nil
		}
	}
	return m.onDemand, nil
}

func (m *Manager) Start(ctx context.Context) error {
	if err := m.static.ManageAsync(ctx, []string{m.host}); err != nil {
		return fmt.Errorf("manage the certificate for %s: %w", m.host, err)
	}
	if err := m.reconcile(ctx); err != nil {
		m.logger.Error("reconcile wildcard certificate", "error", err)
	}
	go m.reconcileLoop(ctx)
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancelWild != nil {
		m.cancelWild()
		m.cancelWild = nil
	}
	m.mu.Unlock()
	m.cache.Stop()
}

func (m *Manager) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.reconcile(ctx); err != nil {
				m.logger.Error("reconcile wildcard certificate", "error", err)
			}
		}
	}
}

func (m *Manager) reconcile(ctx context.Context) error {
	cfg, err := m.settings.Get(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadExistingWildcard(ctx, cfg.BaseDomain)
	desired := ""
	if cfg.BaseDomain != "" && dnsConfigured(cfg) {
		desired = wildcardPrefix + cfg.BaseDomain
	}
	if desired == "" {
		m.stopWildcard()
		return nil
	}
	fingerprint := credentialFingerprint(cfg)
	_, tracked := m.wild.snapshot()
	if desired == m.managed && (fingerprint == m.fingerprint || tracked.State != WildcardFailed) {
		m.fingerprint = fingerprint
		return nil
	}
	return m.startWildcard(ctx, desired, fingerprint)
}

func (m *Manager) startWildcard(ctx context.Context, name, fingerprint string) error {
	m.stopWildcard()
	manageCtx, cancel := context.WithCancel(ctx)
	m.wild.begin(name)
	if err := m.wildcard.ManageAsync(manageCtx, []string{name}); err != nil {
		cancel()
		m.wild.clear()
		return fmt.Errorf("manage the wildcard certificate: %w", err)
	}
	m.cancelWild, m.managed, m.fingerprint = cancel, name, fingerprint
	return nil
}

func (m *Manager) stopWildcard() {
	if m.cancelWild != nil {
		m.cancelWild()
		m.cancelWild = nil
	}
	m.managed, m.fingerprint = "", ""
	m.wild.clear()
}

func (m *Manager) loadExistingWildcard(ctx context.Context, base string) {
	if base == "" || base == m.loadedBase {
		return
	}
	m.loadedBase = base
	if _, err := m.wildcard.CacheManagedCertificate(ctx, wildcardPrefix+base); err != nil {
		m.logger.Debug("no stored wildcard certificate", "error", err)
	}
}

func prepareStorage(path string) error {
	if err := os.MkdirAll(path, storageMode); err != nil {
		return fmt.Errorf("create certificate storage: %w", err)
	}
	if err := os.Chmod(path, storageMode); err != nil {
		return fmt.Errorf("restrict certificate storage: %w", err)
	}
	return nil
}

func loadRoots(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ACME_CA_ROOT: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("ACME_CA_ROOT holds no PEM certificate")
	}
	return pool, nil
}

func portNumber(port string) int {
	number, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return number
}
