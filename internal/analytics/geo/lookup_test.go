package geo

import (
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func writeDatabase(t *testing.T, path string, database []byte) {
	t.Helper()
	if err := os.WriteFile(path, database, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLookupLocatesAddresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	writeDatabase(t, path, buildDatabase(t, "London"))
	l := NewLookup(path, nil)
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if !l.Available() {
		t.Fatal("not available")
	}
	cases := []struct{ addr, country, city string }{
		{"192.0.2.55", "GB", "London"},
		{"::ffff:192.0.2.55", "GB", "London"},
		{"2001:db8::1", "FR", ""},
		{"198.51.100.1", "", ""},
	}
	for _, c := range cases {
		country, city := l.Locate(netip.MustParseAddr(c.addr))
		if country != c.country || city != c.city {
			t.Errorf("Locate(%s) = %q, %q; want %q, %q", c.addr, country, city, c.country, c.city)
		}
	}
	if country, city := l.Locate(netip.Addr{}); country != "" || city != "" {
		t.Errorf("an invalid address gave %q, %q", country, city)
	}
}

func TestLookupWithoutAFileIsUnavailable(t *testing.T) {
	l := NewLookup(filepath.Join(t.TempDir(), FileName), nil)
	if l.Available() {
		t.Error("available without a file")
	}
	if country, city := l.Locate(netip.MustParseAddr("192.0.2.1")); country != "" || city != "" {
		t.Errorf("Locate = %q, %q", country, city)
	}
}

func TestLookupReloadsWhenTheFileChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	l := NewLookup(path, nil)
	clock := time.Now()
	l.now = func() time.Time { return clock }
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	addr := netip.MustParseAddr("192.0.2.9")
	if _, city := l.Locate(addr); city != "" {
		t.Fatalf("city = %q before a file exists", city)
	}
	writeDatabase(t, path, buildDatabase(t, "London"))
	clock = clock.Add(2 * time.Minute)
	if _, city := l.Locate(addr); city != "London" {
		t.Fatalf("city = %q after the file appeared", city)
	}
	writeDatabase(t, path, buildDatabase(t, "Manchester"))
	clock = clock.Add(2 * time.Minute)
	if _, city := l.Locate(addr); city != "Manchester" {
		t.Errorf("city = %q after the file changed", city)
	}
}

func TestLookupSurvivesConcurrentReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	writeDatabase(t, path, buildDatabase(t, "London"))
	l := NewLookup(path, nil)
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				l.Locate(netip.MustParseAddr("192.0.2.9"))
			}
		}()
	}
	for range 20 {
		if err := l.Reload(); err != nil {
			t.Error(err)
		}
	}
	wg.Wait()
}
