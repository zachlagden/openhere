package geo

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/settings"
)

func TestUpdaterDownloadsAndInstallsTheDatabase(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	if e.lookup.Available() {
		t.Fatal("the lookup is available before the first download")
	}
	e.check()
	if last := e.state.last(); last.Status != settings.GeoOK || !last.Updated || last.Detail != "" {
		t.Fatalf("recorded state = %+v", last)
	}
	country, city := e.lookup.Locate(netip.MustParseAddr("192.0.2.9"))
	if country != "GB" || city != "London" {
		t.Errorf("Locate = %q, %q", country, city)
	}
	if !e.lookup.Available() {
		t.Error("the lookup is not available after the download")
	}
	entries, err := os.ReadDir(e.dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != FileName {
		t.Errorf("geo directory = %v, %v; want only %s", entries, err, FileName)
	}
	for _, auth := range e.mm.seenAuth {
		if auth != testAccount+":"+testKey {
			t.Errorf("basic auth = %q", auth)
		}
	}
	e.assertNoSecretLogged()
}

func TestUpdaterRejectsBadCredentials(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	sealed, err := e.box.Seal("wrong-example-key")
	if err != nil {
		t.Fatal(err)
	}
	e.state.cfg.Analytics.MaxMindLicenseKey = sealed
	e.check()
	last := e.state.last()
	if last.Status != settings.GeoFailed || !strings.Contains(last.Detail, "401") || last.Updated {
		t.Errorf("recorded state = %+v", last)
	}
	if e.lookup.Available() {
		t.Error("a failed download made the lookup available")
	}
	e.assertNoSecretLogged("wrong-example-key")
}

func TestUpdaterKeepsTheOldFileOnChecksumMismatch(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.check()
	before, err := os.ReadFile(filepath.Join(e.dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	e.mm.mu.Lock()
	e.mm.archive = buildArchive(t, buildDatabase(t, "Leeds"))
	e.mm.checksum = strings.Repeat("0", 64)
	e.mm.mu.Unlock()
	e.now = e.now.Add(8 * 24 * time.Hour)
	e.check()
	if last := e.state.last(); last.Status != settings.GeoFailed || !strings.Contains(last.Detail, "checksum") {
		t.Errorf("recorded state = %+v", last)
	}
	after, err := os.ReadFile(filepath.Join(e.dir, FileName))
	if err != nil || string(after) != string(before) {
		t.Errorf("the installed database changed after a checksum mismatch: %v", err)
	}
	if _, city := e.lookup.Locate(netip.MustParseAddr("192.0.2.9")); city != "London" {
		t.Errorf("city = %q, want the old London", city)
	}
	assertOnlyDatabase(t, e.dir)
}

func TestUpdaterKeepsTheOldFileOnPartialDownload(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.check()
	e.mm.mu.Lock()
	e.mm.archive = buildArchive(t, buildDatabase(t, "Leeds"))
	e.mm.truncate = true
	e.mm.mu.Unlock()
	e.now = e.now.Add(8 * 24 * time.Hour)
	e.check()
	if last := e.state.last(); last.Status != settings.GeoFailed {
		t.Errorf("recorded state = %+v", last)
	}
	if _, city := e.lookup.Locate(netip.MustParseAddr("192.0.2.9")); city != "London" {
		t.Errorf("city = %q, want the old London", city)
	}
	assertOnlyDatabase(t, e.dir)
}

func TestUpdaterRefusesAnArchiveWithoutADatabase(t *testing.T) {
	e := newUpdaterEnv(t, []byte("not an archive"))
	e.check()
	if last := e.state.last(); last.Status != settings.GeoFailed {
		t.Errorf("recorded state = %+v", last)
	}
	if e.lookup.Available() {
		t.Error("the lookup is available")
	}
	assertOnlyNothing(t, e.dir)
}

func TestUpdaterRefusesAnInvalidDatabase(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, []byte("this is not an mmdb file")))
	e.check()
	last := e.state.last()
	if last.Status != settings.GeoFailed || !strings.Contains(last.Detail, "not a valid") {
		t.Errorf("recorded state = %+v", last)
	}
	assertOnlyNothing(t, e.dir)
}

func TestUpdaterAcceptsADownloadWithoutAChecksum(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.mm.noSum = true
	e.check()
	last := e.state.last()
	if last.Status != settings.GeoOK || !strings.Contains(last.Detail, "not verified") {
		t.Errorf("recorded state = %+v", last)
	}
}

func TestUpdaterFailsOnAChecksumServerError(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.mm.sumStatus = 500
	e.check()
	if last := e.state.last(); last.Status != settings.GeoFailed {
		t.Errorf("recorded state = %+v", last)
	}
	assertOnlyNothing(t, e.dir)
}

func TestUpdaterReportsTheRateLimit(t *testing.T) {
	e := newUpdaterEnv(t, nil)
	e.mm.status = 429
	e.check()
	if last := e.state.last(); !strings.Contains(last.Detail, "429") {
		t.Errorf("recorded state = %+v", last)
	}
}

func TestUpdaterDoesNothingWithoutCredentials(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.state.cfg.Analytics.MaxMindLicenseKey = ""
	e.check()
	if e.mm.requestCount() != 0 {
		t.Errorf("MaxMind was contacted %d times without a key", e.mm.requestCount())
	}
	e.state.cfg.Analytics.GeoStatus = settings.GeoOK
	e.check()
	if last := e.state.last(); last.Status != settings.GeoUnconfigured {
		t.Errorf("recorded state = %+v", last)
	}
}

func TestUpdaterRefreshesAtMostWeekly(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.check()
	first := e.mm.requestCount()
	e.now = e.now.Add(6 * 24 * time.Hour)
	e.check()
	if e.mm.requestCount() != first {
		t.Error("the database was refreshed before a week passed")
	}
	e.state.cfg.Analytics.GeoUpdatedAt = e.now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	e.check()
	if e.mm.requestCount() == first {
		t.Error("the database was not refreshed after a week")
	}
}

func TestUpdaterBacksOffAfterAFailure(t *testing.T) {
	e := newUpdaterEnv(t, nil)
	e.mm.status = 500
	e.check()
	failed := e.mm.requestCount()
	e.check()
	if e.mm.requestCount() != failed {
		t.Error("the updater retried straight away")
	}
	e.now = e.now.Add(7 * time.Hour)
	e.state.cfg.Analytics.GeoCheckedAt = e.now.Add(-7 * time.Hour).Format(time.RFC3339)
	e.check()
	if e.mm.requestCount() == failed {
		t.Error("the updater did not retry after the back-off")
	}
}

func TestUpdaterRetriesAtOnceWhenTheKeyChanges(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	wrong, err := e.box.Seal("wrong-example-key")
	if err != nil {
		t.Fatal(err)
	}
	right := e.state.cfg.Analytics.MaxMindLicenseKey
	e.state.cfg.Analytics.MaxMindLicenseKey = wrong
	e.check()
	if e.state.last().Status != settings.GeoFailed {
		t.Fatal("the first attempt did not fail")
	}
	e.state.cfg.Analytics.MaxMindLicenseKey = right
	e.check()
	if e.state.last().Status != settings.GeoOK {
		t.Errorf("recorded state = %+v", e.state.last())
	}
}

func TestUpdaterRefusesPlainHTTPRedirects(t *testing.T) {
	e := newUpdaterEnv(t, nil)
	e.mm.redirect = "http://example.com/database.tar.gz"
	e.check()
	last := e.state.last()
	if last.Status != settings.GeoFailed || !strings.Contains(last.Detail, "not HTTPS") {
		t.Errorf("recorded state = %+v", last)
	}
}

func assertOnlyDatabase(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != FileName {
		t.Errorf("geo directory = %v, %v; want only %s", entries, err, FileName)
	}
}

func assertOnlyNothing(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Errorf("geo directory = %v, %v; want it empty", entries, err)
	}
}
