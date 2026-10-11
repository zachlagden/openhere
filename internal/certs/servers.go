package certs

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mholt/acmez/v3"

	"github.com/zachlagden/openhere/internal/httptimeouts"
)

const (
	httpIdleTimeout = 30 * time.Second
	httpBodyTimeout = 10 * time.Second
	maxHeaderBytes  = 1 << 16
)

func (m *Manager) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           m.httpIssuer.HTTPChallengeHandler(redirectHandler(m.httpsPort)),
		ReadHeaderTimeout: httptimeouts.DefaultReadHeader,
		ReadTimeout:       httpBodyTimeout,
		WriteTimeout:      httpBodyTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          quietLog(),
	}
}

func (m *Manager) HTTPSServer(addr string, handler http.Handler, timeouts httptimeouts.Timeouts) *http.Server {
	srv := timeouts.OrDefaults().NewServer(addr, handler)
	srv.TLSConfig = m.TLSConfig()
	srv.MaxHeaderBytes = maxHeaderBytes
	srv.ErrorLog = quietLog()
	return srv
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
