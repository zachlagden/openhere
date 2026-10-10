package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/agentcode"
	"github.com/zachlagden/selfhost/internal/analytics"
	"github.com/zachlagden/selfhost/internal/analytics/geo"
	"github.com/zachlagden/selfhost/internal/api"
	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/hashlimit"
	"github.com/zachlagden/selfhost/internal/hosting"
	"github.com/zachlagden/selfhost/internal/invites"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/passwordreset"
	"github.com/zachlagden/selfhost/internal/policy"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/publish"
	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/sitefiles"
	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/telemetry"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

const (
	shutdownTimeout   = 15 * time.Second
	readHeaderTimeout = 10 * time.Second
	startupTimeout    = 30 * time.Second

	maintenanceInterval   = time.Hour
	wildcardCheckInterval = 6 * time.Hour
	geoCheckInterval      = 5 * time.Minute
	analyticsFlushTimeout = 20 * time.Second
	geoDirName            = "geo"
	setupPath             = "/api/v1/setup"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "selfhost:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}

	logger := telemetry.NewLogger(os.Stdout, cfg.LogLevel)
	slog.SetDefault(logger)

	flushSentry, sentryEnabled, err := telemetry.InitSentry(cfg.SentryDSN, version)
	if err != nil {
		return err
	}
	defer flushSentry()

	if err := serve(cfg, logger); err != nil {
		if sentryEnabled {
			sentry.CaptureException(err)
		}
		logger.Error("server stopped with an error", "error", err)
		return err
	}
	return nil
}

func serve(cfg config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := openStore(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer closeStore(db, logger)

	app, err := buildApp(ctx, cfg, db, logger)
	if err != nil {
		return err
	}
	defer app.stop()

	return runServers(ctx, app.servers(cfg), app.start, logger)
}

func buildApp(ctx context.Context, cfg config.Config, db store.Store, logger *slog.Logger) (*application, error) {
	hashlimit.SetDefault(hashlimit.New(cfg.Argon2Concurrency, hashlimit.DefaultWait))
	svc, err := auth.New(db, auth.Options{KeyPrefix: cfg.APIKeyPrefix, Logger: logger})
	if err != nil {
		return nil, err
	}
	gate, err := newSetupGate(ctx, svc, logger)
	if err != nil {
		return nil, err
	}
	files, err := sitefiles.New(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	box, err := secrets.Resolve(cfg.SecretKey, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load secret key: %w", err)
	}
	resolver := hosting.NewResolver(db, nil)
	settingsSvc := settings.New(db, settings.Options{DefaultBaseDomain: cfg.PublicURL.Host, Secrets: box})
	domainSvc := domains.New(db, settingsSvc, domains.Options{ManagementHost: cfg.PublicURL.Host, Logger: logger})
	certManager, err := newCertManager(cfg, settingsSvc, box, domainSvc, logger)
	if err != nil {
		return nil, err
	}
	siteChanged := func(slug string) {
		resolver.Invalidate(slug)
		domainSvc.Invalidate()
	}
	mailer := mail.New(mail.NewSettingsSource(settingsSvc, box), mail.Options{Logger: logger})
	workspaceSvc := workspaces.New(db, workspaces.Options{MayCreateShared: policy.MayCreateShared(settingsSvc, svc)})
	accessSvc := access.New(db, access.Options{Mailer: mailer, Logger: logger, Secrets: box})
	siteSvc := sites.New(db, workspaceSvc, sites.Options{
		Files: files, Logger: logger, OnSiteChange: siteChanged, DefaultAccessMode: defaultAccessMode(settingsSvc), AnalyticsDefault: analyticsDefault(settingsSvc),
	})
	publishSvc := publish.New(db, siteSvc, workspaceSvc, settingsSvc, files, publish.Options{Logger: logger, OnSiteChange: siteChanged})
	baseURL := cfg.PublicURL.String()
	invitesSvc := invites.New(db, svc, workspaceSvc, settingsSvc, invites.Options{BaseURL: baseURL, Mailer: mailer, Logger: logger})
	resetSvc := passwordreset.New(db, svc, passwordreset.Options{BaseURL: baseURL, Mailer: mailer, Logger: logger})
	agentSvc := agentcode.New(db, svc, box, agentcode.Options{BaseURL: baseURL, Mailer: mailer, Logger: logger})
	probes := hosting.NewProbes(nil)
	checker := hosting.NewChecker(settingsSvc, probes, hosting.CheckerOptions{Public: cfg.PublicURL, Logger: logger})
	geoDir := filepath.Join(cfg.DataDir, geoDirName)
	locator := geo.NewLookup(filepath.Join(geoDir, geo.FileName), logger)
	recorder := analytics.NewRecorder(db, analytics.RecorderOptions{
		Hasher: analytics.NewHasher(db, box, logger), Locator: locator, Logger: logger,
	})
	recorder.Start()
	analyticsSvc := analytics.New(db, siteSvc, workspaceSvc, settingsSvc, analytics.Options{Recorder: recorder, Location: locator})
	geoUpdater := geo.NewUpdater(settingsSvc, box, locator, geo.UpdaterOptions{Dir: geoDir, Logger: logger})
	server := api.New(svc, workspaceSvc, siteSvc, publishSvc, settingsSvc, accessSvc, gate, api.Options{
		TrustedProxies: cfg.TrustedProxies, Logger: logger, Public: cfg.PublicURL, Wildcard: checker,
		Mailer: mailer, Invites: invitesSvc, PasswordReset: resetSvc, AgentCodes: agentSvc, Domains: domainSvc,
		TLS: tlsReporter(certManager), Analytics: analyticsSvc, Version: version,
	})
	dashboard, err := newDashboard(logger)
	if err != nil {
		return nil, err
	}
	router := hosting.New(dashboard.Wrap(server.Handler()), settingsSvc, db, files,
		hosting.Options{
			Public: cfg.PublicURL, Probes: probes, Resolver: resolver, Logger: logger, Access: accessSvc,
			Proxies: proxy.Trust{Prefixes: cfg.TrustedProxies}, Domains: domainSvc, HSTS: cfg.TLS.Enabled, Views: recorder,
		})

	stop := startBackground(ctx, svc, checker, geoUpdater,
		auth.MaintenanceTask{Name: "publish", Run: publishSvc.Maintain},
		auth.MaintenanceTask{Name: "domains", Run: domainSvc.Maintain},
		auth.MaintenanceTask{Name: "access", Run: accessSvc.Purge},
		auth.MaintenanceTask{Name: "password reset", Run: resetSvc.Purge},
		auth.MaintenanceTask{Name: "agent codes", Run: agentSvc.Purge},
		auth.MaintenanceTask{Name: "analytics", Run: analyticsSvc.Maintain})
	closeAnalytics := func() { shutdownAnalytics(recorder, locator, logger) }
	return &application{handler: router, certs: certManager, stopBackground: stop, closeAnalytics: closeAnalytics}, nil
}

func startBackground(ctx context.Context, svc *auth.Service, checker *hosting.Checker, geoUpdater *geo.Updater, tasks ...auth.MaintenanceTask) func() {
	backgroundCtx, cancel := context.WithCancel(ctx)
	var running sync.WaitGroup
	running.Add(3)
	go func() {
		defer running.Done()
		svc.RunMaintenance(backgroundCtx, maintenanceInterval, tasks...)
	}()
	go func() {
		defer running.Done()
		checker.Run(backgroundCtx, wildcardCheckInterval)
	}()
	go func() {
		defer running.Done()
		geoUpdater.Run(backgroundCtx, geoCheckInterval)
	}()
	return func() {
		cancel()
		running.Wait()
	}
}

func defaultAccessMode(settingsSvc *settings.Service) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		cfg, err := settingsSvc.Get(ctx)
		if err != nil {
			return "", err
		}
		return cfg.DefaultAccessMode, nil
	}
}

func shutdownAnalytics(recorder *analytics.Recorder, locator *geo.Lookup, logger *slog.Logger) {
	flushCtx, cancel := context.WithTimeout(context.Background(), analyticsFlushTimeout)
	defer cancel()
	if err := recorder.Close(flushCtx); err != nil {
		logger.Error("flush analytics events", "error", err)
	}
	if err := locator.Close(); err != nil {
		logger.Error("close the GeoLite2 database", "error", err)
	}
}

func analyticsDefault(settingsSvc *settings.Service) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		cfg, err := settingsSvc.Get(ctx)
		if err != nil {
			return false, err
		}
		return cfg.Analytics.EnabledByDefault, nil
	}
}

func newSetupGate(ctx context.Context, svc *auth.Service, logger *slog.Logger) (*auth.SetupGate, error) {
	startCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	needed, err := svc.SetupNeeded(startCtx)
	if err != nil {
		return nil, fmt.Errorf("check setup state: %w", err)
	}
	if !needed {
		return auth.ClosedSetupGate(), nil
	}
	gate, err := auth.NewSetupGate()
	if err != nil {
		return nil, fmt.Errorf("create setup token: %w", err)
	}
	logger.Info("setup required: send this token with the first setup request",
		"setup_path", setupPath, "setup_token", gate.Token())
	return gate, nil
}

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (store.Store, error) {
	startCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	db, err := store.Open(startCtx, store.Options{DatabaseURL: cfg.DatabaseURL, DataDir: cfg.DataDir})
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := db.Migrate(startCtx); err != nil {
		closeStore(db, logger)
		return nil, fmt.Errorf("migrate store: %w", err)
	}
	logger.Info("store ready", "driver", string(db.Driver()))
	return db, nil
}

func closeStore(db store.Store, logger *slog.Logger) {
	if err := db.Close(); err != nil {
		logger.Error("close store", "error", err)
	}
}
