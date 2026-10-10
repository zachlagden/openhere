package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/zachlagden/selfhost/internal/api"
	"github.com/zachlagden/selfhost/internal/certs"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/ui"
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
		return []*http.Server{{
			Addr:              cfg.ListenAddr,
			Handler:           a.handler,
			ReadHeaderTimeout: readHeaderTimeout,
		}}
	}
	return []*http.Server{
		a.certs.HTTPServer(cfg.TLS.HTTPAddr),
		a.certs.HTTPSServer(cfg.TLS.HTTPSAddr, a.handler),
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
