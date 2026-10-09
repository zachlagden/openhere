package hosting

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	ProbePath     = "/.well-known/selfhost-check/"
	probeLifetime = 60 * time.Second
	tokenBytes    = 16
)

type Probes struct {
	now func() time.Time

	mu     sync.Mutex
	tokens map[string]time.Time
}

func NewProbes(now func() time.Time) *Probes {
	if now == nil {
		now = time.Now
	}
	return &Probes{now: now, tokens: map[string]time.Time{}}
}

func (p *Probes) Issue() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate probe token: %w", err)
	}
	token := hex.EncodeToString(raw)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dropExpired()
	p.tokens[token] = p.now().Add(probeLifetime)
	return token, nil
}

func (p *Probes) Consume(token string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	expiresAt, ok := p.tokens[token]
	if !ok {
		return false
	}
	delete(p.tokens, token)
	return p.now().Before(expiresAt)
}

func (p *Probes) Revoke(token string) {
	p.mu.Lock()
	delete(p.tokens, token)
	p.mu.Unlock()
}

func (p *Probes) dropExpired() {
	now := p.now()
	for token, expiresAt := range p.tokens {
		if !now.Before(expiresAt) {
			delete(p.tokens, token)
		}
	}
}
