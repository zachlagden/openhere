package geo

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	FileName      = "GeoLite2-City.mmdb"
	recheckEvery  = time.Minute
	englishNames  = "en"
	countryLength = 2
)

type cityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

type Lookup struct {
	path   string
	logger *slog.Logger
	now    func() time.Time

	lastCheck atomic.Int64

	mu      sync.RWMutex
	reader  *maxminddb.Reader
	modTime time.Time
	size    int64
}

func NewLookup(path string, logger *slog.Logger) *Lookup {
	if logger == nil {
		logger = slog.Default()
	}
	l := &Lookup{path: path, logger: logger, now: time.Now}
	if err := l.Reload(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Warn("open the GeoLite2 database", "error", err)
	}
	return l
}

func (l *Lookup) Available() bool {
	l.maybeReload()
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.reader != nil
}

func (l *Lookup) Locate(addr netip.Addr) (string, string) {
	l.maybeReload()
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.reader == nil || !addr.IsValid() {
		return "", ""
	}
	result := l.reader.Lookup(addr.Unmap())
	if !result.Found() {
		return "", ""
	}
	var record cityRecord
	if err := result.Decode(&record); err != nil {
		return "", ""
	}
	country := record.Country.ISOCode
	if len(country) != countryLength {
		country = ""
	}
	return country, record.City.Names[englishNames]
}

func (l *Lookup) Reload() error {
	info, err := os.Stat(l.path)
	if err != nil {
		return fmt.Errorf("stat the GeoLite2 database: %w", err)
	}
	reader, err := maxminddb.Open(l.path)
	if err != nil {
		return fmt.Errorf("open the GeoLite2 database: %w", err)
	}
	l.mu.Lock()
	previous := l.reader
	l.reader, l.modTime, l.size = reader, info.ModTime(), info.Size()
	if previous != nil {
		if err := previous.Close(); err != nil {
			l.logger.Warn("close the previous GeoLite2 database", "error", err)
		}
	}
	l.mu.Unlock()
	return nil
}

func (l *Lookup) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.reader == nil {
		return nil
	}
	err := l.reader.Close()
	l.reader = nil
	return err
}

func (l *Lookup) maybeReload() {
	now := l.now().UnixNano()
	last := l.lastCheck.Load()
	if now-last < int64(recheckEvery) || !l.lastCheck.CompareAndSwap(last, now) {
		return
	}
	if !l.changed() {
		return
	}
	if err := l.Reload(); err != nil {
		l.logger.Warn("reload the GeoLite2 database", "error", err)
	}
}

func (l *Lookup) changed() bool {
	info, err := os.Stat(l.path)
	if err != nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.reader == nil || !info.ModTime().Equal(l.modTime) || info.Size() != l.size
}
