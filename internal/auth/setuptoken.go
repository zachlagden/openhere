package auth

import (
	"crypto/subtle"
	"sync"
)

type SetupGate struct {
	mu    sync.RWMutex
	token string
}

func NewSetupGate() (*SetupGate, error) {
	token, err := NewToken()
	if err != nil {
		return nil, err
	}
	return &SetupGate{token: token}, nil
}

func ClosedSetupGate() *SetupGate { return &SetupGate{} }

func (g *SetupGate) Token() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token
}

func (g *SetupGate) Check(candidate string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(g.token), []byte(candidate)) == 1
}

func (g *SetupGate) Discard() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.token = ""
}
