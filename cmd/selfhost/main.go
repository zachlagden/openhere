package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/agentcode"
	"github.com/zachlagden/selfhost/internal/api"
	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
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

	handler, stopMaintenance, err := buildHandler(ctx, cfg, db, logger)
	if err != nil {
		return err
	}
	defer stopMaintenance()

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}
	return runServer(ctx, srv, logger)
}

func buildHandler(ctx context.Context, cfg config.Config, db store.Store, logger *slog.Logger) (http.Handler, func(), error) {
	svc, err := auth.New(db, auth.Options{KeyPrefix: cfg.APIKeyPrefix, Logger: logger})
	if err != nil {
		return nil, nil, err
	}
	gate, err := newSetupGate(ctx, svc, logger)
	if err != nil {
		return nil, nil, err
	}
	files, err := sitefiles.New(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}
	box, err := secrets.Resolve(cfg.SecretKey, cfg.DataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load secret key: %w", err)
	}
	resolver := hosting.NewResolver(db, nil)
	settingsSvc := settings.New(db, settings.Options{DefaultBaseDomain: cfg.PublicURL.Host, Secrets: box})
	mailer := mail.New(mail.NewSettingsSource(settingsSvc, box), mail.Options{Logger: logger})
	workspaceSvc := workspaces.New(db, workspaces.Options{MayCreateShared: policy.MayCreateShared(settingsSvc, svc)})
	accessSvc := access.New(db, access.Options{Mailer: mailer, Logger: logger})
	siteSvc := sites.New(db, workspaceSvc, sites.Options{
		Files: files, Logger: logger, OnSiteChange: resolver.Invalidate, DefaultAccessMode: defaultAccessMode(settingsSvc),
	})
	publishSvc := publish.New(db, siteSvc, workspaceSvc, settingsSvc, files, publish.Options{Logger: logger, OnSiteChange: resolver.Invalidate})
	baseURL := cfg.PublicURL.String()
	invitesSvc := invites.New(db, svc, workspaceSvc, settingsSvc, invites.Options{BaseURL: baseURL, Mailer: mailer, Logger: logger})
	resetSvc := passwordreset.New(db, svc, passwordreset.Options{BaseURL: baseURL, Mailer: mailer, Logger: logger})
	agentSvc := agentcode.New(db, svc, box, agentcode.Options{BaseURL: baseURL, Mailer: mailer, Logger: logger})
	probes := hosting.NewProbes(nil)
	checker := hosting.NewChecker(settingsSvc, probes, hosting.CheckerOptions{Public: cfg.PublicURL, Logger: logger})
	server := api.New(svc, workspaceSvc, siteSvc, publishSvc, settingsSvc, accessSvc, gate, api.Options{
		TrustedProxies: cfg.TrustedProxies, Logger: logger, Public: cfg.PublicURL, Wildcard: checker,
		Mailer: mailer, Invites: invitesSvc, PasswordReset: resetSvc, AgentCodes: agentSvc,
	})
	router := hosting.New(server.Handler(), settingsSvc, db, files,
		hosting.Options{
			Public: cfg.PublicURL, Probes: probes, Resolver: resolver, Logger: logger, Access: accessSvc,
			Proxies: proxy.Trust{Prefixes: cfg.TrustedProxies},
		})

	stop := startBackground(ctx, svc, checker,
		auth.MaintenanceTask{Name: "publish", Run: publishSvc.Maintain},
		auth.MaintenanceTask{Name: "access", Run: accessSvc.Purge},
		auth.MaintenanceTask{Name: "password reset", Run: resetSvc.Purge},
		auth.MaintenanceTask{Name: "agent codes", Run: agentSvc.Purge})
	return router, stop, nil
}

func startBackground(ctx context.Context, svc *auth.Service, checker *hosting.Checker, tasks ...auth.MaintenanceTask) func() {
	backgroundCtx, cancel := context.WithCancel(ctx)
	var running sync.WaitGroup
	running.Add(2)
	go func() {
		defer running.Done()
		svc.RunMaintenance(backgroundCtx, maintenanceInterval, tasks...)
	}()
	go func() {
		defer running.Done()
		checker.Run(backgroundCtx, wildcardCheckInterval)
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

func runServer(ctx context.Context, srv *http.Server, logger *slog.Logger) error {
	failed := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
		close(failed)
	}()

	select {
	case err := <-failed:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("stopped")
	return nil
}
