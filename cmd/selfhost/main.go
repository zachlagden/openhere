package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/zachlagden/selfhost/internal/api"
	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/publish"
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

	maintenanceInterval = time.Hour
	setupPath           = "/api/v1/setup"
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
	settingsSvc := settings.New(db, settings.Options{})
	workspaceSvc := workspaces.New(db, workspaces.Options{})
	siteSvc := sites.New(db, workspaceSvc, sites.Options{Files: files, Logger: logger})
	publishSvc := publish.New(db, siteSvc, workspaceSvc, settingsSvc, files, publish.Options{Logger: logger})
	server := api.New(svc, workspaceSvc, siteSvc, publishSvc, settingsSvc, gate, api.Options{TrustedProxies: cfg.TrustedProxies, Logger: logger})

	maintenanceCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.RunMaintenance(maintenanceCtx, maintenanceInterval,
			auth.MaintenanceTask{Name: "publish", Run: publishSvc.Maintain})
	}()
	stop := func() {
		cancel()
		<-done
	}
	return server.Handler(), stop, nil
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
