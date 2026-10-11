package geo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/zachlagden/openhere/internal/secrets"
	"github.com/zachlagden/openhere/internal/settings"
)

const (
	testAccount = "123456"
	testKey     = "licence-example-key"
)

func buildDatabase(t *testing.T, city string) []byte {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "GeoLite2-City", RecordSize: 24, IncludeReservedNetworks: true})
	if err != nil {
		t.Fatal(err)
	}
	_, network, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	record := mmdbtype.Map{
		"country": mmdbtype.Map{"iso_code": mmdbtype.String("GB")},
		"city":    mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(city)}},
	}
	if err := writer.Insert(network, record); err != nil {
		t.Fatal(err)
	}
	_, v6, err := net.ParseCIDR("2001:db8::/32")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Insert(v6, mmdbtype.Map{"country": mmdbtype.Map{"iso_code": mmdbtype.String("FR")}}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := writer.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func buildArchive(t *testing.T, database []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zipper := gzip.NewWriter(&out)
	archive := tar.NewWriter(zipper)
	entries := []struct {
		name string
		body []byte
	}{
		{"GeoLite2-City_20260101/COPYRIGHT.txt", []byte("example")},
		{"GeoLite2-City_20260101/" + FileName, database},
	}
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type fakeMaxMind struct {
	mu        sync.Mutex
	archive   []byte
	checksum  string
	noSum     bool
	status    int
	truncate  bool
	requests  int
	seenAuth  []string
	server    *httptest.Server
	sumStatus int
	redirect  string
	block     chan struct{}
	entered   chan struct{}
}

func newFakeMaxMind(t *testing.T, archive []byte) *fakeMaxMind {
	t.Helper()
	f := &fakeMaxMind{archive: archive}
	sum := sha256.Sum256(archive)
	f.checksum = hex.EncodeToString(sum[:])
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeMaxMind) handle(w http.ResponseWriter, r *http.Request) {
	if f.block != nil {
		f.entered <- struct{}{}
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	user, pass, _ := r.BasicAuth()
	f.seenAuth = append(f.seenAuth, user+":"+pass)
	if f.redirect != "" {
		http.Redirect(w, r, f.redirect, http.StatusFound)
		return
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}
	if user != testAccount || pass != testKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/geoip/databases/GeoLite2-City/download" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.URL.Query().Get("suffix") {
	case "tar.gz":
		f.serveArchive(w)
	case "tar.gz.sha256":
		f.serveChecksum(w)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeMaxMind) serveArchive(w http.ResponseWriter) {
	if f.truncate {
		w.Header().Set("Content-Length", "100000")
		if _, err := w.Write(f.archive[:len(f.archive)/2]); err != nil {
			return
		}
		return
	}
	if _, err := w.Write(f.archive); err != nil {
		return
	}
}

func (f *fakeMaxMind) serveChecksum(w http.ResponseWriter) {
	if f.noSum {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if f.sumStatus != 0 {
		w.WriteHeader(f.sumStatus)
		return
	}
	if _, err := io.WriteString(w, f.checksum+"  GeoLite2-City_20260101.tar.gz\n"); err != nil {
		return
	}
}

func (f *fakeMaxMind) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

type fakeState struct {
	mu       sync.Mutex
	cfg      settings.Settings
	recorded []settings.GeoState
}

func (s *fakeState) Get(context.Context) (settings.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, nil
}

func (s *fakeState) RecordGeo(ctx context.Context, state settings.GeoState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded = append(s.recorded, state)
	s.cfg.Analytics.GeoStatus = state.Status
	s.cfg.Analytics.GeoDetail = state.Detail
	stamp := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	s.cfg.Analytics.GeoCheckedAt = stamp
	if state.Updated {
		s.cfg.Analytics.GeoUpdatedAt = stamp
	}
	return nil
}

func (s *fakeState) last() settings.GeoState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.recorded) == 0 {
		return settings.GeoState{}
	}
	return s.recorded[len(s.recorded)-1]
}

type updaterEnv struct {
	t       *testing.T
	dir     string
	mm      *fakeMaxMind
	state   *fakeState
	lookup  *Lookup
	updater *Updater
	box     *secrets.Box
	logs    *bytes.Buffer
	now     time.Time
}

func newUpdaterEnv(t *testing.T, archive []byte) *updaterEnv {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{3}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal(testKey)
	if err != nil {
		t.Fatal(err)
	}
	e := &updaterEnv{t: t, dir: filepath.Join(t.TempDir(), "geo"), mm: newFakeMaxMind(t, archive), box: box, logs: &bytes.Buffer{}}
	e.now = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	e.state = &fakeState{cfg: settings.Defaults()}
	e.state.cfg.Analytics.MaxMindAccountID = testAccount
	e.state.cfg.Analytics.MaxMindLicenseKey = sealed
	logger := slog.New(slog.NewTextHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e.lookup = NewLookup(filepath.Join(e.dir, FileName), logger)
	e.updater = NewUpdater(e.state, box, e.lookup, UpdaterOptions{
		Dir: e.dir, BaseURL: e.mm.server.URL, HTTPClient: e.mm.server.Client(), Logger: logger, Now: func() time.Time { return e.now },
	})
	return e
}

func (e *updaterEnv) check() {
	e.t.Helper()
	if err := e.updater.Check(context.Background()); err != nil {
		e.t.Fatalf("Check: %v", err)
	}
}

func (e *updaterEnv) assertNoSecretLogged(extra ...string) {
	e.t.Helper()
	all := e.logs.String()
	for _, s := range e.state.recorded {
		all += s.Detail
	}
	for _, secret := range append([]string{testKey}, extra...) {
		if strings.Contains(all, secret) {
			e.t.Errorf("the output contains %q", secret)
		}
	}
}

func (f *fakeMaxMind) setStatus(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}
