package analytics

import (
	"bytes"
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const (
	docIP = "192.0.2.77"
	docUA = "Mozilla/5.0 (X11; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0"
)

func TestVisitorHashRules(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		ip := netip.MustParseAddr(docIP)
		noon := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
		base, err := e.hasher.Visitor(ctx, ip, docUA, "site-a", noon)
		if err != nil {
			t.Fatal(err)
		}
		again, err := e.hasher.Visitor(ctx, ip, docUA, "site-a", noon.Add(9*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if base != again {
			t.Error("the same visitor on the same day and site got two hashes")
		}
		if len(base) != 32 || strings.Contains(base, docIP) {
			t.Errorf("hash = %q", base)
		}
		variants := map[string]struct {
			ip   string
			ua   string
			site string
			when time.Time
		}{
			"next day":   {docIP, docUA, "site-a", noon.Add(24 * time.Hour)},
			"other site": {docIP, docUA, "site-b", noon},
			"other ip":   {"192.0.2.78", docUA, "site-a", noon},
			"other ua":   {docIP, docUA + " Extra", "site-a", noon},
		}
		for name, v := range variants {
			got, err := e.hasher.Visitor(ctx, netip.MustParseAddr(v.ip), v.ua, v.site, v.when)
			if err != nil {
				t.Fatal(err)
			}
			if got == base {
				t.Errorf("%s: the hash did not change", name)
			}
		}
	})
}

func TestVisitorHashChangesWithTheSecret(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		ctx := context.Background()
		ip := netip.MustParseAddr(docIP)
		when := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
		first, err := e.hasher.Visitor(ctx, ip, docUA, "site-a", when)
		if err != nil {
			t.Fatal(err)
		}
		sameStored := NewHasher(db, e.box, nil)
		got, err := sameStored.Visitor(ctx, ip, docUA, "site-a", when)
		if err != nil || got != first {
			t.Errorf("a second hasher over the same secret gave %q, %v; want %q", got, err, first)
		}
		var stored string
		if err := db.QueryRowContext(ctx, "SELECT value FROM instance_settings WHERE key = $1", secretSettingKey).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(stored, "v1:") {
			t.Errorf("the stored secret is not sealed: %q", stored)
		}
		otherBox, err := secrets.New(bytes.Repeat([]byte{9}, secrets.KeyBytes))
		if err != nil {
			t.Fatal(err)
		}
		replaced, err := NewHasher(db, otherBox, nil).Visitor(ctx, ip, docUA, "site-a", when)
		if err != nil {
			t.Fatalf("a secret that cannot be opened must be replaced, got %v", err)
		}
		if replaced == first {
			t.Error("a replaced secret produced the old hash")
		}
	})
}
