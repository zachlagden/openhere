package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/config"
	"github.com/zachlagden/openhere/internal/httptimeouts"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func answering(body string) *http.Server {
	return &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}),
		ReadHeaderTimeout: time.Second,
	}
}

func fetch(t *testing.T, addr string) (string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func TestBindAllClosesEarlierListenersOnFailure(t *testing.T) {
	taken, err := listen(t)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()
	first := answering("one")
	second := answering("two")
	second.Addr = taken.Addr().String()

	if _, err := bindAll(t.Context(), []*http.Server{first, second}); err == nil {
		t.Fatal("binding an address in use succeeded")
	}
	probe, err := listen(t)
	if err != nil {
		t.Fatalf("listeners leaked: %v", err)
	}
	_ = probe.Close()
}

func TestRunServersServesAndStopsEveryListener(t *testing.T) {
	servers := []*http.Server{answering("one"), answering("two")}
	bound, err := bindAll(t.Context(), servers)
	if err != nil {
		t.Fatal(err)
	}
	addrs := []string{bound[0].ln.Addr().String(), bound[1].ln.Addr().String()}
	closeAll(bound)
	servers[0].Addr, servers[1].Addr = addrs[0], addrs[1]

	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runServers(ctx, servers, func(context.Context) error { close(started); return nil }, quietLogger())
	}()
	<-started

	for i, want := range []string{"one", "two"} {
		var got string
		var fetchErr error
		for range 50 {
			if got, fetchErr = fetch(t, addrs[i]); fetchErr == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if fetchErr != nil || got != want {
			t.Errorf("server %d answered %q, %v", i, got, fetchErr)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runServers = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runServers did not return after shutdown")
	}
	for i, addr := range addrs {
		if _, err := fetch(t, addr); err == nil {
			t.Errorf("server %d still answers after shutdown", i)
		}
	}
}

func TestRunServersStopsWhenReadyFails(t *testing.T) {
	servers := []*http.Server{answering("one")}
	failure := errors.New("manager failed")
	err := runServers(t.Context(), servers, func(context.Context) error { return failure }, quietLogger())
	if !errors.Is(err, failure) {
		t.Errorf("error = %v", err)
	}
}

func listen(t *testing.T) (net.Listener, error) {
	t.Helper()
	var listenConfig net.ListenConfig
	return listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
}

func TestThePlainListenerCarriesEveryTimeout(t *testing.T) {
	app := &application{handler: http.NotFoundHandler(), timeouts: httptimeouts.Defaults()}
	servers := app.servers(config.Config{ListenAddr: ":8080"})
	if len(servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(servers))
	}
	srv := servers[0]
	if srv.ReadHeaderTimeout != 10*time.Second || srv.ReadTimeout != 30*time.Second || srv.WriteTimeout != 60*time.Second || srv.IdleTimeout != 90*time.Second {
		t.Errorf("timeouts = %v %v %v %v", srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != maxHeaderBytes || srv.Addr != ":8080" {
		t.Errorf("server = %+v", srv)
	}
}
