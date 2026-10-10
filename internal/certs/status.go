package certs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"

	"github.com/zachlagden/selfhost/internal/settings"
)

const (
	WildcardNone    = "none"
	WildcardPending = "pending"
	WildcardOK      = "ok"
	WildcardFailed  = "failed"

	maxStatusDomains = 5000

	reasonIssueFailed = "The wildcard certificate could not be issued. Check the DNS provider token and the server log."
	reasonNoProvider  = "No DNS provider is configured, so no wildcard certificate is requested."
	reasonNoBase      = "No base domain is set."
)

type WildcardStatus struct {
	State  string
	Reason string
}

type CustomSummary struct {
	Count         int
	SoonestExpiry *time.Time
}

type Status struct {
	Enabled               bool
	ManagementExpiresAt   *time.Time
	Wildcard              WildcardStatus
	Custom                CustomSummary
	DNSProviderConfigured bool
}

type wildcardTracker struct {
	mu     sync.Mutex
	name   string
	state  string
	reason string
}

func (t *wildcardTracker) begin(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.name, t.state, t.reason = name, WildcardPending, ""
}

func (t *wildcardTracker) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.name, t.state, t.reason = "", WildcardNone, ""
}

func (t *wildcardTracker) record(name, state, reason string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if name != t.name {
		return false
	}
	if state == WildcardPending && t.state == WildcardFailed {
		return true
	}
	t.state, t.reason = state, reason
	return true
}

func (t *wildcardTracker) snapshot() (name string, status WildcardStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.name, WildcardStatus{State: t.state, Reason: t.reason}
}

func (m *Manager) onWildcardEvent(_ context.Context, event string, data map[string]any) error {
	identifier, _ := data["identifier"].(string)
	switch event {
	case "cert_obtaining":
		m.wild.record(identifier, WildcardPending, "")
	case "cert_obtained":
		m.wild.record(identifier, WildcardOK, "")
	case "cert_failed":
		if !m.wild.record(identifier, WildcardFailed, reasonIssueFailed) {
			return nil
		}
		m.logger.Error("wildcard certificate failed", "identifier", identifier, "error", fmt.Sprint(data["error"]))
	}
	return nil
}

func (m *Manager) Status(ctx context.Context) (Status, error) {
	cfg, err := m.settings.Get(ctx)
	if err != nil {
		return Status{}, err
	}
	status := Status{
		Enabled:               true,
		ManagementExpiresAt:   m.expiryOf(m.host),
		Wildcard:              m.wildcardStatus(cfg),
		DNSProviderConfigured: dnsConfigured(cfg),
	}
	names, err := m.domains.VerifiedNames(ctx, maxStatusDomains)
	if err != nil {
		return Status{}, err
	}
	status.Custom = m.customSummary(names)
	return status, nil
}

func (m *Manager) wildcardStatus(cfg settings.Settings) WildcardStatus {
	if cfg.BaseDomain != "" && m.expiryOf("*."+cfg.BaseDomain) != nil {
		return WildcardStatus{State: WildcardOK}
	}
	name, tracked := m.wild.snapshot()
	switch {
	case name != "":
		return tracked
	case cfg.BaseDomain == "":
		return WildcardStatus{State: WildcardNone, Reason: reasonNoBase}
	default:
		return WildcardStatus{State: WildcardNone, Reason: reasonNoProvider}
	}
}

func (m *Manager) customSummary(names []string) CustomSummary {
	var summary CustomSummary
	for _, name := range names {
		expiry := m.expiryOf(name)
		if expiry == nil {
			continue
		}
		summary.Count++
		if summary.SoonestExpiry == nil || expiry.Before(*summary.SoonestExpiry) {
			summary.SoonestExpiry = expiry
		}
	}
	return summary
}

func (m *Manager) expiryOf(name string) *time.Time {
	var latest *time.Time
	for _, cert := range m.cache.AllMatchingCertificates(name) {
		if !coversExactly(cert, name) || cert.Leaf == nil || !m.now().Before(cert.Leaf.NotAfter) {
			continue
		}
		expiry := cert.Leaf.NotAfter.UTC()
		if latest == nil || expiry.After(*latest) {
			latest = &expiry
		}
	}
	return latest
}

func coversExactly(cert certmagic.Certificate, name string) bool {
	for _, candidate := range cert.Names {
		if candidate == name {
			return true
		}
	}
	return false
}
