package geo

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zachlagden/openhere/internal/fsperm"
	"github.com/zachlagden/openhere/internal/secrets"
	"github.com/zachlagden/openhere/internal/settings"
)

const (
	DefaultBaseURL = "https://download.maxmind.com"
	editionID      = "GeoLite2-City"
	refreshAfter   = 7 * 24 * time.Hour
	retryAfter     = 6 * time.Hour
	restartRetry   = 5 * time.Minute
	forceFloor     = time.Minute
	downloadWindow = 15 * time.Minute
	recordTimeout  = 10 * time.Second
)

type State interface {
	Get(ctx context.Context) (settings.Settings, error)
	RecordGeo(ctx context.Context, state settings.GeoState) error
}

type UpdaterOptions struct {
	Dir        string
	BaseURL    string
	HTTPClient *http.Client
	Now        func() time.Time
	Logger     *slog.Logger
}

type Updater struct {
	state  State
	box    *secrets.Box
	lookup *Lookup
	dir    string
	base   string
	client *http.Client
	now    func() time.Time
	logger *slog.Logger

	mu          sync.Mutex
	fingerprint string

	forceMu    sync.Mutex
	lastForced time.Time
	nudges     chan struct{}
}

type TooSoonError struct {
	RetryAfter time.Duration
}

func (e *TooSoonError) Error() string {
	seconds := int(math.Ceil(e.RetryAfter.Seconds()))
	return fmt.Sprintf("a check was started less than a minute ago, try again in %d seconds", seconds)
}

func NewUpdater(state State, box *secrets.Box, lookup *Lookup, opts UpdaterOptions) *Updater {
	u := &Updater{
		state: state, box: box, lookup: lookup, dir: opts.Dir, base: strings.TrimRight(opts.BaseURL, "/"),
		client: opts.HTTPClient, now: opts.Now, logger: opts.Logger, nudges: make(chan struct{}, 1),
	}
	if u.base == "" {
		u.base = DefaultBaseURL
	}
	if u.now == nil {
		u.now = time.Now
	}
	if u.logger == nil {
		u.logger = slog.Default()
	}
	if u.client == nil {
		u.client = &http.Client{Timeout: downloadWindow}
	}
	checkRedirect := u.client.CheckRedirect
	u.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return errors.New("refused a redirect to a URL that is not HTTPS")
		}
		if checkRedirect != nil {
			return checkRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return u
}

func (u *Updater) Run(ctx context.Context, interval time.Duration) {
	u.logCheck(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.logCheck(ctx)
		case <-u.nudges:
			u.runNudged(ctx)
		}
	}
}

func (u *Updater) Nudge() {
	select {
	case u.nudges <- struct{}{}:
	default:
	}
}

func (u *Updater) runNudged(ctx context.Context) {
	for {
		err := u.ForceCheck(ctx)
		var soon *TooSoonError
		if !errors.As(err, &soon) {
			if err != nil {
				u.logger.Error("check the GeoLite2 database", "error", err)
			}
			return
		}
		timer := time.NewTimer(soon.RetryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		select {
		case <-u.nudges:
		default:
		}
	}
}

func (u *Updater) ForceCheck(ctx context.Context) error {
	if err := u.reserveForce(); err != nil {
		return err
	}
	return u.check(ctx, true)
}

func (u *Updater) reserveForce() error {
	u.forceMu.Lock()
	defer u.forceMu.Unlock()
	now := u.now()
	if wait := forceFloor - now.Sub(u.lastForced); !u.lastForced.IsZero() && wait > 0 {
		return &TooSoonError{RetryAfter: wait}
	}
	u.lastForced = now
	return nil
}

func (u *Updater) logCheck(ctx context.Context) {
	if err := u.Check(ctx); err != nil {
		u.logger.Error("check the GeoLite2 database", "error", err)
	}
}

func (u *Updater) Check(ctx context.Context) error {
	return u.check(ctx, false)
}

func (u *Updater) check(ctx context.Context, forced bool) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	cfg, err := u.state.Get(ctx)
	if err != nil {
		return err
	}
	account, key, err := u.credentials(cfg)
	if err != nil {
		return u.record(ctx, settings.GeoState{Status: settings.GeoFailed, Detail: "The stored licence key cannot be opened. Enter it again."})
	}
	if account == "" || key == "" {
		return u.recordUnconfigured(ctx, cfg)
	}
	if !u.due(cfg, account+":"+key, forced) {
		return nil
	}
	return u.refresh(ctx, account, key)
}

func (u *Updater) credentials(cfg settings.Settings) (string, string, error) {
	account := cfg.Analytics.MaxMindAccountID
	sealed := cfg.Analytics.MaxMindLicenseKey
	if account == "" || sealed == "" {
		return "", "", nil
	}
	key, err := u.box.Open(sealed)
	if err != nil {
		return "", "", err
	}
	return account, key, nil
}

func (u *Updater) record(ctx context.Context, state settings.GeoState) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	return u.state.RecordGeo(ctx, state)
}

func (u *Updater) recordUnconfigured(ctx context.Context, cfg settings.Settings) error {
	if cfg.Analytics.GeoStatus == settings.GeoUnconfigured {
		return nil
	}
	return u.record(ctx, settings.GeoState{Status: settings.GeoUnconfigured})
}

func (u *Updater) due(cfg settings.Settings, fingerprint string, forced bool) bool {
	now := u.now()
	if u.hasFile() {
		updated, err := time.Parse(time.RFC3339, cfg.Analytics.GeoUpdatedAt)
		if err == nil && now.Sub(updated) < refreshAfter {
			return false
		}
	}
	if cfg.Analytics.GeoStatus != settings.GeoFailed || forced {
		return true
	}
	checked, err := time.Parse(time.RFC3339, cfg.Analytics.GeoCheckedAt)
	if err != nil {
		return true
	}
	if u.fingerprint == "" {
		return now.Sub(checked) >= restartRetry
	}
	return u.fingerprint != fingerprint || now.Sub(checked) >= retryAfter
}

func (u *Updater) hasFile() bool {
	_, err := os.Stat(filepath.Join(u.dir, FileName))
	return err == nil
}

func (u *Updater) refresh(ctx context.Context, account, key string) error {
	u.fingerprint = account + ":" + key
	if err := fsperm.EnsureDir(u.dir, u.logger); err != nil {
		return u.fail(ctx, fmt.Errorf("create the geo directory: %w", err))
	}
	checksumNote, err := u.download(ctx, account, key)
	if err != nil {
		return u.fail(ctx, err)
	}
	if err := u.lookup.Reload(); err != nil {
		return u.fail(ctx, err)
	}
	u.logger.Info("updated the GeoLite2 database")
	return u.record(ctx, settings.GeoState{Status: settings.GeoOK, Detail: checksumNote, Updated: true})
}

func (u *Updater) fail(ctx context.Context, cause error) error {
	u.logger.Warn("update the GeoLite2 database", "reason", cause.Error())
	if err := u.record(ctx, settings.GeoState{Status: settings.GeoFailed, Detail: cause.Error()}); err != nil {
		return errors.Join(cause, err)
	}
	return nil
}

func (u *Updater) download(ctx context.Context, account, key string) (string, error) {
	archive, sum, err := u.fetchArchive(ctx, account, key)
	if err != nil {
		return "", err
	}
	defer discard(archive)
	note, err := u.verify(ctx, account, key, sum)
	if err != nil {
		return "", err
	}
	if err := u.install(archive); err != nil {
		return "", err
	}
	return note, nil
}

func (u *Updater) verify(ctx context.Context, account, key string, sum []byte) (string, error) {
	expected, available, err := u.fetchChecksum(ctx, account, key)
	if err != nil {
		return "", err
	}
	if !available {
		return "MaxMind published no checksum for this download, so it was not verified.", nil
	}
	if hex.EncodeToString(sum) != expected {
		return "", errors.New("the downloaded archive does not match MaxMind's checksum")
	}
	return "", nil
}

func removeQuietly(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Debug("remove a temporary geo file", "error", err)
	}
}
