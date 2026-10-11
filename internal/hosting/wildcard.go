package hosting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zachlagden/openhere/internal/config"
	"github.com/zachlagden/openhere/internal/settings"
)

const (
	probeTimeout    = 10 * time.Second
	maxProbeBody    = 256
	maxDetailLength = 500
	recordTimeout   = 10 * time.Second
	labelBytes      = 6

	scheduledAttempts = 3
	attemptGap        = 10 * time.Second
	startupDelay      = 5 * time.Second
	RetryInterval     = 15 * time.Minute
)

type HostResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type CheckerOptions struct {
	Public   config.PublicURL
	Resolver HostResolver
	Client   Doer
	Timeout  time.Duration
	Logger   *slog.Logger
	Sleep    func(ctx context.Context, d time.Duration) error
}

type Outcome struct {
	Verified bool
	Detail   string
}

type Checker struct {
	settings *settings.Service
	probes   *Probes
	public   config.PublicURL
	resolver HostResolver
	client   Doer
	timeout  time.Duration
	logger   *slog.Logger
	sleep    func(ctx context.Context, d time.Duration) error

	mu sync.Mutex
}

func NewChecker(settingsSvc *settings.Service, probes *Probes, opts CheckerOptions) *Checker {
	resolver := opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = probeTimeout
	}
	client := opts.Client
	if client == nil {
		client = newProbeClient(timeout, nil)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	return &Checker{
		settings: settingsSvc, probes: probes, public: opts.Public,
		resolver: resolver, client: client, timeout: timeout, logger: logger, sleep: sleep,
	}
}

func newProbeClient(timeout time.Duration, transport http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (c *Checker) Check(ctx context.Context) (settings.Settings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	outcome, err := c.probe(ctx)
	if err != nil {
		return settings.Settings{}, err
	}
	return c.record(ctx, outcome)
}

func (c *Checker) Run(ctx context.Context, interval time.Duration) {
	if err := c.sleep(ctx, startupDelay); err != nil {
		return
	}
	for {
		wait := interval
		if c.runScheduled(ctx) {
			wait = RetryInterval
		}
		if err := c.sleep(ctx, wait); err != nil {
			return
		}
	}
}

func needsRetry(cfg settings.Settings) bool {
	return cfg.WildcardStatus == settings.WildcardFailed && (cfg.SubdomainEnabled || cfg.SubdomainDisabledReason != "")
}

func (c *Checker) runScheduled(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	outcome, ok := c.probeWithRetries(ctx)
	if !ok {
		return false
	}
	recorded, err := c.settings.RecordScheduledWildcard(ctx, outcome.Verified, truncate(outcome.Detail, maxDetailLength))
	if err != nil {
		c.logger.Error("record wildcard check", "error", err)
		return false
	}
	c.logger.Info("wildcard check finished", "status", recorded.WildcardStatus, "detail", recorded.WildcardDetail,
		"consecutiveFailures", recorded.WildcardFailures, "subdomainEnabled", recorded.SubdomainEnabled)
	return needsRetry(recorded)
}

func (c *Checker) probeWithRetries(ctx context.Context) (Outcome, bool) {
	var outcome Outcome
	for attempt := 1; attempt <= scheduledAttempts; attempt++ {
		var err error
		outcome, err = c.probe(ctx)
		if err != nil {
			c.logger.Error("wildcard check", "error", err)
			return Outcome{}, false
		}
		if outcome.Verified || attempt == scheduledAttempts {
			break
		}
		if err := c.sleep(ctx, attemptGap); err != nil {
			return Outcome{}, false
		}
	}
	return outcome, true
}

func (c *Checker) record(ctx context.Context, outcome Outcome) (settings.Settings, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	return c.settings.RecordWildcard(ctx, outcome.Verified, truncate(outcome.Detail, maxDetailLength))
}

func (c *Checker) probe(ctx context.Context) (Outcome, error) {
	cfg, err := c.settings.Get(ctx)
	if err != nil {
		return Outcome{}, err
	}
	if cfg.BaseDomain == "" {
		return failure("No base domain is set. Set hosting.baseDomain first."), nil
	}
	label, err := randomLabel()
	if err != nil {
		return Outcome{}, err
	}
	host := label + "." + cfg.BaseDomain
	addrs, err := c.resolver.LookupHost(ctx, host)
	if err != nil || len(addrs) == 0 {
		return failure(fmt.Sprintf("DNS did not resolve %s. Add a wildcard record such as *.%s that points to this server.", host, cfg.BaseDomain)), nil
	}
	token := c.probes.Issue()
	defer c.probes.Revoke(token)
	return c.request(ctx, host, token), nil
}

func (c *Checker) request(ctx context.Context, host, token string) Outcome {
	target := c.public.Scheme + "://" + joinPort(host, c.public.Port) + ProbePath + token
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return failure("The check could not build its request: " + err.Error())
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return failure(fmt.Sprintf("The request to %s failed: %s", host, err.Error()))
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProbeBody))
	closeErr := resp.Body.Close()
	return judge(host, c.probes.Proof(token), resp.StatusCode, body, errors.Join(readErr, closeErr))
}

func judge(host, proof string, status int, body []byte, readErr error) Outcome {
	switch {
	case status >= 300 && status < 400:
		return failure(fmt.Sprintf("%s answered with a redirect. The check does not follow redirects. Serve the wildcard host directly from this server.", host))
	case readErr != nil:
		return failure(fmt.Sprintf("The response from %s could not be read: %s", host, readErr.Error()))
	case status != http.StatusOK || strings.TrimSpace(string(body)) != proof:
		return failure(fmt.Sprintf("%s answered, but not with this server's check token (HTTP %d). Check that the wildcard record points to this server and that your reverse proxy forwards the Host header.", host, status))
	}
	return Outcome{Verified: true}
}

func failure(detail string) Outcome {
	return Outcome{Detail: detail}
}

func joinPort(host, port string) string {
	if port == "" {
		return host
	}
	return host + ":" + port
}

func randomLabel() (string, error) {
	raw := make([]byte, labelBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate probe label: %w", err)
	}
	return "check-" + hex.EncodeToString(raw), nil
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.ToValidUTF8(text[:limit], "")
}
