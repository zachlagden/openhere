package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/zachlagden/selfhost/internal/validate"
)

const (
	KeyEmailProvider  = "email.provider"
	KeyEmailFrom      = "email.fromAddress"
	KeyEmailFromName  = "email.fromName"
	KeySMTPHost       = "email.smtp.host"
	KeySMTPPort       = "email.smtp.port"
	KeySMTPSecurity   = "email.smtp.security"
	KeySMTPUsername   = "email.smtp.username"
	KeySMTPPassword   = "email.smtp.password"
	KeyResendAPIKey   = "email.resend.apiKey"
	secretSetSuffix   = "Set"
	ProviderNone      = "none"
	ProviderSMTP      = "smtp"
	ProviderResend    = "resend"
	SecurityTLS       = "tls"
	SecurityStartTLS  = "starttls"
	SecurityNone      = "none"
	defaultSMTPPort   = 587
	maxSecretLength   = 1024
	maxUsernameLength = 255
	maxFromNameLength = 100
	maxAddressLength  = 254
	maxHostLength     = 253
	maxPort           = 65535
)

type Email struct {
	Provider     string
	FromAddress  string
	FromName     string
	SMTPHost     string
	SMTPPort     int
	SMTPSecurity string
	SMTPUsername string
	SMTPPassword string
	ResendAPIKey string
}

func emailDefinitions() []definition {
	return []definition{
		enumDefinition(KeyEmailProvider, ProviderNone, []string{ProviderNone, ProviderSMTP, ProviderResend},
			func(s *Settings, v string) { s.Email.Provider = v },
			func(s Settings) string { return s.Email.Provider }),
		stringDefinition(KeyEmailFrom, "", normaliseFromAddress,
			func(s *Settings, v string) { s.Email.FromAddress = v },
			func(s Settings) string { return s.Email.FromAddress }),
		stringDefinition(KeyEmailFromName, "", normaliseFromName,
			func(s *Settings, v string) { s.Email.FromName = v },
			func(s Settings) string { return s.Email.FromName }),
		stringDefinition(KeySMTPHost, "", normaliseSMTPHost,
			func(s *Settings, v string) { s.Email.SMTPHost = v },
			func(s Settings) string { return s.Email.SMTPHost }),
		intDefinition(KeySMTPPort, defaultSMTPPort, maxPort,
			func(s *Settings, v int64) { s.Email.SMTPPort = int(v) },
			func(s Settings) int64 { return int64(s.Email.SMTPPort) }),
		enumDefinition(KeySMTPSecurity, SecurityStartTLS, []string{SecurityTLS, SecurityStartTLS, SecurityNone},
			func(s *Settings, v string) { s.Email.SMTPSecurity = v },
			func(s Settings) string { return s.Email.SMTPSecurity }),
		stringDefinition(KeySMTPUsername, "", normaliseUsername,
			func(s *Settings, v string) { s.Email.SMTPUsername = v },
			func(s Settings) string { return s.Email.SMTPUsername }),
		secretDefinition(KeySMTPPassword,
			func(s *Settings, v string) { s.Email.SMTPPassword = v },
			func(s Settings) string { return s.Email.SMTPPassword }),
		secretDefinition(KeyResendAPIKey,
			func(s *Settings, v string) { s.Email.ResendAPIKey = v },
			func(s Settings) string { return s.Email.ResendAPIKey }),
	}
}

func enumDefinition(key, fallback string, allowed []string, set func(*Settings, string), get func(Settings) string) definition {
	choices := strings.Join(allowed, ", ")
	normalise := func(raw string) (string, error) {
		value := strings.TrimSpace(raw)
		for _, option := range allowed {
			if value == option {
				return value, nil
			}
		}
		return "", fmt.Errorf("use one of %s", choices)
	}
	return stringDefinition(key, fallback, normalise, set, get)
}

func secretDefinition(key string, set func(*Settings, string), get func(Settings) string) definition {
	return definition{
		key:    key,
		secret: true,
		outKey: key + secretSetSuffix,
		decode: func(raw json.RawMessage) (string, error) {
			var value string
			if err := decodeValue(key, raw, &value); err != nil {
				return "", err
			}
			if value == "" {
				return "", nil
			}
			if utf8.RuneCountInString(value) > maxSecretLength || hasControl(value) {
				return "", validate.Invalid(fmt.Sprintf("Setting %s must be at most %d characters without control characters.", key, maxSecretLength))
			}
			return value, nil
		},
		apply: func(s *Settings, stored string) error {
			set(s, stored)
			return nil
		},
		value: func(s Settings) any { return get(s) != "" },
	}
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func normaliseFromAddress(raw string) (string, error) {
	address := strings.ToLower(strings.TrimSpace(raw))
	if address == "" {
		return "", nil
	}
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || len(address) > maxAddressLength || !strings.Contains(address[strings.LastIndex(address, "@"):], ".") {
		return "", errors.New("enter a plain email address such as noreply@example.com")
	}
	return address, nil
}

func normaliseFromName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) > maxFromNameLength || hasControl(name) {
		return "", fmt.Errorf("use at most %d characters and no line breaks", maxFromNameLength)
	}
	return name, nil
}

func normaliseUsername(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) > maxUsernameLength || hasControl(name) {
		return "", fmt.Errorf("use at most %d characters and no line breaks", maxUsernameLength)
	}
	return name, nil
}

func normaliseSMTPHost(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" {
		return "", nil
	}
	if len(host) > maxHostLength {
		return "", fmt.Errorf("the host is longer than %d characters", maxHostLength)
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}
	for _, label := range strings.Split(host, ".") {
		if !hostnameLabel.MatchString(label) {
			return "", errors.New("use a bare host name or IP address, without a scheme, port or path")
		}
	}
	return host, nil
}
