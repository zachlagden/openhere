package httptimeouts

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func shortTimeouts() Timeouts {
	return Timeouts{
		ReadHeader: 200 * time.Millisecond, Read: 200 * time.Millisecond, Write: 200 * time.Millisecond, Idle: 200 * time.Millisecond,
		Base: 3 * time.Second, Rate: 1000, Cap: 10 * time.Second,
	}
}

func startServer(t *testing.T, timeouts Timeouts, handler http.Handler) string {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	timeouts.Apply(srv.Config)
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

func TestDefaultsMatchTheDocumentedValues(t *testing.T) {
	d := Defaults()
	if d.ReadHeader != 10*time.Second || d.Read != 30*time.Second || d.Write != 60*time.Second || d.Idle != 90*time.Second {
		t.Errorf("defaults = %+v", d)
	}
	srv := &http.Server{}
	d.Apply(srv)
	if srv.ReadHeaderTimeout != d.ReadHeader || srv.ReadTimeout != d.Read || srv.WriteTimeout != d.Write || srv.IdleTimeout != d.Idle {
		t.Errorf("Apply left the server with %+v", srv)
	}
	if (Timeouts{}).OrDefaults() != d {
		t.Error("a zero Timeouts did not fall back to the defaults")
	}
}

func TestExtensionGrowsWithSizeAndStopsAtTheCap(t *testing.T) {
	d := Defaults()
	cases := map[int64]time.Duration{
		-1:        time.Minute,
		0:         time.Minute,
		2 << 20:   time.Minute + time.Second,
		200 << 20: time.Minute + 100*time.Second,
		1 << 40:   30 * time.Minute,
	}
	for size, want := range cases {
		if got := d.Extension(size); got != want {
			t.Errorf("Extension(%d) = %v, want %v", size, got, want)
		}
	}
}

func TestSlowBodyIsClosedAfterTheReadTimeout(t *testing.T) {
	failed := make(chan error, 1)
	addr := startServer(t, shortTimeouts(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		failed <- err
	}))
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 100\r\n\r\nabc"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-failed:
		if err == nil {
			t.Error("the handler read a complete body from a stalled client")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a stalled body was not cut off")
	}
}

func TestSteadySlowUploadCompletesWithinItsExtendedDeadline(t *testing.T) {
	timeouts := shortTimeouts()
	received := make(chan int, 1)
	addr := startServer(t, timeouts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeouts.ExtendUpload(w, r.ContentLength)
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		received <- int(n)
		time.Sleep(250 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	}))
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	chunks := 8
	if _, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 800\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	for range chunks {
		time.Sleep(100 * time.Millisecond)
		if _, err := io.WriteString(conn, strings.Repeat("x", 100)); err != nil {
			t.Fatalf("the server closed a steady upload: %v", err)
		}
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "done" || <-received != 800 {
		t.Errorf("status %d, body %q", resp.StatusCode, body)
	}
}

func TestWithoutExtensionTheSameSlowUploadIsCutOff(t *testing.T) {
	addr := startServer(t, shortTimeouts(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}))
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 800\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	cut := false
	for range 8 {
		time.Sleep(100 * time.Millisecond)
		if _, err := io.WriteString(conn, strings.Repeat("x", 100)); err != nil {
			cut = true
			break
		}
	}
	if !cut {
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			cut = resp.StatusCode != http.StatusOK
		} else {
			cut = true
		}
	}
	if !cut {
		t.Error("the slow upload survived without an extension")
	}
}

func TestSlowDownloadSurvivesOnceTheWriteDeadlineIsExtended(t *testing.T) {
	timeouts := shortTimeouts()
	addr := startServer(t, timeouts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeouts.ExtendDownload(w, 4000)
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(w, "payload")
	}))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "payload" {
		t.Errorf("body %q, err %v", body, err)
	}
}

func TestIdleConnectionsAreClosed(t *testing.T) {
	addr := startServer(t, shortTimeouts(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("idle connection read = %v, want EOF", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("idle connection lasted %v", time.Since(start))
	}
}

func TestExtendersTolerateWritersWithoutDeadlines(t *testing.T) {
	rec := httptest.NewRecorder()
	Defaults().ExtendUpload(rec, 10)
	Defaults().ExtendDownload(rec, 10)
}
