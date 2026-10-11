package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
)

type boundServer struct {
	srv *http.Server
	ln  net.Listener
}

func bindAll(ctx context.Context, servers []*http.Server) ([]boundServer, error) {
	var listenConfig net.ListenConfig
	bound := make([]boundServer, 0, len(servers))
	for _, srv := range servers {
		ln, err := listenConfig.Listen(ctx, "tcp", srv.Addr)
		if err != nil {
			closeAll(bound)
			return nil, fmt.Errorf("listen on %s: %w", srv.Addr, err)
		}
		bound = append(bound, boundServer{srv: srv, ln: ln})
	}
	return bound, nil
}

func closeAll(bound []boundServer) {
	for _, b := range bound {
		if err := b.ln.Close(); err != nil {
			slog.Debug("close listener", "error", err)
		}
	}
}

func runServers(ctx context.Context, servers []*http.Server, ready func(context.Context) error, logger *slog.Logger) error {
	bound, err := bindAll(ctx, servers)
	if err != nil {
		return err
	}
	if err := ready(ctx); err != nil {
		closeAll(bound)
		return err
	}
	failed := serveAll(bound, logger)
	select {
	case err := <-failed:
		shutdownErr := shutdownAll(bound, logger)
		if err != nil {
			return fmt.Errorf("serve: %w", errors.Join(err, shutdownErr))
		}
		return shutdownErr
	case <-ctx.Done():
		logger.Info("shutting down")
	}
	if err := shutdownAll(bound, logger); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("stopped")
	return nil
}

func serveAll(bound []boundServer, logger *slog.Logger) <-chan error {
	failed := make(chan error, len(bound))
	var running sync.WaitGroup
	for _, b := range bound {
		running.Add(1)
		go func() {
			defer running.Done()
			logger.Info("listening", "addr", b.ln.Addr().String(), "tls", b.srv.TLSConfig != nil)
			if err := serveOne(b); err != nil && !errors.Is(err, http.ErrServerClosed) {
				failed <- err
			}
		}()
	}
	go func() {
		running.Wait()
		close(failed)
	}()
	return failed
}

func serveOne(b boundServer) error {
	if b.srv.TLSConfig != nil {
		return b.srv.ServeTLS(b.ln, "", "")
	}
	return b.srv.Serve(b.ln)
}

func shutdownAll(bound []boundServer, logger *slog.Logger) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	results := make(chan error, len(bound))
	for _, b := range bound {
		go func() { results <- b.srv.Shutdown(shutdownCtx) }()
	}
	var problems []error
	for range bound {
		if err := <-results; err != nil {
			problems = append(problems, err)
		}
	}
	if err := errors.Join(problems...); err != nil {
		logger.Error("shutdown", "error", err)
		return err
	}
	return nil
}
