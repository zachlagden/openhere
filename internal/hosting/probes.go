package hosting

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const (
	ProbePath     = "/.well-known/selfhost-check/"
	probeLifetime = 60 * time.Second
)

type probe struct {
	proof     string
	expiresAt time.Time
}

type Probes struct {
	now func() time.Time
	key []byte

	mu     sync.Mutex
	tokens map[string]probe
}

func NewProbes(now func() time.Time) *Probes {
	if now == nil {
		now = time.Now
	}
	return &Probes{now: now, key: []byte(rand.Text() + rand.Text()), tokens: map[string]probe{}}
}

func (p *Probes) Issue() string {
	token := rand.Text()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dropExpired()
	p.tokens[token] = probe{proof: p.Proof(token), expiresAt: p.now().Add(probeLifetime)}
	return token
}

func (p *Probes) Proof(token string) string {
	mac := hmac.New(sha256.New, p.key)
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (p *Probes) Consume(token string) (string, bool) {
	p.mu.Lock()
	entry, ok := p.tokens[token]
	delete(p.tokens, token)
	p.mu.Unlock()
	if !ok || !p.now().Before(entry.expiresAt) {
		return "", false
	}
	return entry.proof, true
}

func (p *Probes) Revoke(token string) {
	p.mu.Lock()
	delete(p.tokens, token)
	p.mu.Unlock()
}

func (p *Probes) dropExpired() {
	now := p.now()
	for token, entry := range p.tokens {
		if !now.Before(entry.expiresAt) {
			delete(p.tokens, token)
		}
	}
}
