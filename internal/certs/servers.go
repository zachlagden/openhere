package certs

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mholt/acmez/v3"
)

const (
	headerTimeout   = 10 * time.Second
	httpIdleTimeout = 30 * time.Second
	tlsIdleTimeout  = 2 * time.Minute
	httpBodyTimeout = 10 * time.Second
	maxHeaderBytes  = 1 << 16
)

func (m *Manager) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           m.httpIssuer.HTTPChallengeHandler(redirectHandler(m.httpsPort)),
		ReadHeaderTimeout: headerTimeout,
		ReadTimeout:       httpBodyTimeout,
		WriteTimeout:      httpBodyTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          quietLog(),
	}
}

func (m *Manager) HTTPSServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		TLSConfig:         m.TLSConfig(),
		ReadHeaderTimeout: headerTimeout,
		IdleTimeout:       tlsIdleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          quietLog(),
	}
}

func (m *Manager) TLSConfig() *tls.Config {
	config := m.onDemand.TLSConfig()
	config.MinVersion = tls.VersionTLS12
	config.NextProtos = []string{"h2", "http/1.1", acmez.ACMETLS1Protocol}
	return config
}

func quietLog() *log.Logger {
	return log.New(io.Discard, "", 0)
}
