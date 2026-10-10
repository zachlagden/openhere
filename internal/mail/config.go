package mail

import (
	"context"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/settings"
)

const maxPort = 65535

type SMTPConfig struct {
	Host     string
	Port     int
	Security string
	Username string
	Password string
}

type Config struct {
	Provider     string
	FromAddress  string
	FromName     string
	SMTP         SMTPConfig
	ResendAPIKey string
}

func (c Config) Configured() bool {
	if c.FromAddress == "" {
		return false
	}
	switch c.Provider {
	case settings.ProviderSMTP:
		return c.smtpValid()
	case settings.ProviderResend:
		return c.ResendAPIKey != ""
	default:
		return false
	}
}

func (c Config) smtpValid() bool {
	s := c.SMTP
	if s.Host == "" || s.Port < 1 || s.Port > maxPort {
		return false
	}
	switch s.Security {
	case settings.SecurityTLS, settings.SecurityStartTLS, settings.SecurityNone:
	default:
		return false
	}
	return s.Username == "" || s.Password != ""
}

type ConfigSource interface {
	MailConfig(ctx context.Context) (Config, error)
}

type SettingsSource struct {
	settings *settings.Service
	box      *secrets.Box
}

func NewSettingsSource(svc *settings.Service, box *secrets.Box) *SettingsSource {
	return &SettingsSource{settings: svc, box: box}
}

func (s *SettingsSource) MailConfig(ctx context.Context) (Config, error) {
	current, err := s.settings.Get(ctx)
	if err != nil {
		return Config{}, err
	}
	e := current.Email
	password, err := s.open(e.SMTPPassword)
	if err != nil {
		return Config{}, fmt.Errorf("open the SMTP password: %w", err)
	}
	apiKey, err := s.open(e.ResendAPIKey)
	if err != nil {
		return Config{}, fmt.Errorf("open the Resend API key: %w", err)
	}
	return Config{
		Provider:    e.Provider,
		FromAddress: e.FromAddress,
		FromName:    e.FromName,
		SMTP: SMTPConfig{
			Host: e.SMTPHost, Port: e.SMTPPort, Security: e.SMTPSecurity, Username: e.SMTPUsername, Password: password,
		},
		ResendAPIKey: apiKey,
	}, nil
}

func (s *SettingsSource) open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	if s.box == nil {
		return "", errors.New("no secret key is loaded")
	}
	return s.box.Open(sealed)
}
