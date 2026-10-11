package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/zachlagden/openhere/internal/api"
	"github.com/zachlagden/openhere/internal/certs"
	"github.com/zachlagden/openhere/internal/config"
	"github.com/zachlagden/openhere/internal/domains"
	"github.com/zachlagden/openhere/internal/httptimeouts"
	"github.com/zachlagden/openhere/internal/secrets"
	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/ui"
)

func newDashboard(logger *slog.Logger) (*ui.Shell, error) {
	files, err := ui.Dist()
	if err != nil {
		return nil, err
	}
	shell, err := ui.New(files)
	if err != nil {
		return nil, err
	}
	if !shell.Built() {
		logger.Warn("the dashboard is not built into this binary; run \"pnpm --dir web build\" and rebuild")
	}
	return shell, nil
}

type application struct {
	handler        http.Handler
	certs          *certs.Manager
	stopBackground func()
	closeAnalytics func()
	timeouts       httptimeouts.Timeouts
}

func newCertManager(cfg config.Config, settingsSvc *settings.Service, box *secrets.Box, domainSvc *domains.Service, logger *slog.Logger) (*certs.Manager, error) {
	if !cfg.TLS.Enabled {
		return nil, nil
	}
	return certs.New(certs.Options{
		TLS: cfg.TLS, Public: cfg.PublicURL, Settings: settingsSvc, Secrets: box, Domains: domainSvc, Logger: logger,
	})
}

func tlsReporter(manager *certs.Manager) api.TLSReporter {
	if manager == nil {
		return nil
	}
	return manager
}

func (a *application) servers(cfg config.Config) []*http.Server {
	if a.certs == nil {
		srv := a.timeouts.NewServer(cfg.ListenAddr, a.handler)
		srv.MaxHeaderBytes = maxHeaderBytes
		return []*http.Server{srv}
	}
	return []*http.Server{
		a.certs.HTTPServer(cfg.TLS.HTTPAddr),
		a.certs.HTTPSServer(cfg.TLS.HTTPSAddr, a.handler, a.timeouts),
	}
}

func (a *application) start(ctx context.Context) error {
	if a.certs == nil {
		return nil
	}
	return a.certs.Start(ctx)
}

func (a *application) stop() {
	if a.certs != nil {
		a.certs.Stop()
	}
	a.stopBackground()
	a.closeAnalytics()
}
