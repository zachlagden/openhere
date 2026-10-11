package mail

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	"github.com/zachlagden/openhere/internal/settings"
)

const (
	defaultTimeout = 30 * time.Second
	configTimeout  = 5 * time.Second
)

type Options struct {
	Logger         *slog.Logger
	TLSConfig      *tls.Config
	ResendEndpoint string
	Timeout        time.Duration
	Now            func() time.Time
}

type Dynamic struct {
	source ConfigSource
	logger *slog.Logger
	smtp   smtpSender
	resend resendSender
}

func New(source ConfigSource, opts Options) *Dynamic {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	endpoint := opts.ResendEndpoint
	if endpoint == "" {
		endpoint = DefaultResendEndpoint
	}
	return &Dynamic{
		source: source,
		logger: logger,
		smtp:   smtpSender{logger: logger, tlsConfig: opts.TLSConfig, timeout: timeout, now: now},
		resend: newResendSender(logger, endpoint, timeout),
	}
}

func (d *Dynamic) Configured() bool {
	ctx, cancel := context.WithTimeout(context.Background(), configTimeout)
	defer cancel()
	cfg, err := d.source.MailConfig(ctx)
	if err != nil {
		d.logger.Error("read email settings", "error", err)
		return false
	}
	return cfg.Configured()
}

func (d *Dynamic) Send(ctx context.Context, msg Message) error {
	cfg, err := d.source.MailConfig(ctx)
	if err != nil {
		d.logger.Error("read email settings", "error", err)
		return sendError("the saved email settings cannot be read; enter the secrets again", err)
	}
	if !cfg.Configured() {
		return ErrNotConfigured
	}
	switch cfg.Provider {
	case settings.ProviderSMTP:
		return d.report(d.smtp.send(ctx, cfg, msg))
	default:
		return d.report(d.resend.send(ctx, cfg, msg))
	}
}

func (d *Dynamic) report(err error) error {
	if err != nil {
		d.logger.Warn("send email", "reason", Reason(err))
	}
	return err
}
