package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"

	"github.com/zachlagden/selfhost/internal/secrets"
)

const (
	defaultListenAddr = ":8080"
	defaultDataDir    = "./data"
	defaultLogLevel   = "info"
	defaultKeyPrefix  = "sk_"
)

var keyPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)

type Config struct {
	ListenAddr  string
	DataDir     string
	DatabaseURL string
	LogLevel    slog.Level
	SentryDSN   string

	APIKeyPrefix   string
	TrustedProxies []netip.Prefix
	PublicURL      PublicURL
	SecretKey      []byte
}

type Lookup func(key string) (string, bool)

func Load(lookup Lookup) (Config, error) {
	values := reader{lookup: lookup}

	cfg := Config{
		ListenAddr:  values.get("LISTEN_ADDR", defaultListenAddr),
		DataDir:     values.get("DATA_DIR", defaultDataDir),
		DatabaseURL: values.get("DATABASE_URL", ""),
		SentryDSN:   values.get("SENTRY_DSN", ""),

		APIKeyPrefix: values.get("API_KEY_PREFIX", defaultKeyPrefix),
	}

	var problems []error

	level, err := parseLogLevel(values.get("LOG_LEVEL", defaultLogLevel))
	problems = appendIf(problems, err)
	cfg.LogLevel = level

	problems = appendIf(problems, validateListenAddr(cfg.ListenAddr))
	problems = appendIf(problems, validateDatabaseURL(cfg.DatabaseURL))
	problems = appendIf(problems, validateSentryDSN(cfg.SentryDSN))
	problems = appendIf(problems, validateKeyPrefix(cfg.APIKeyPrefix))

	proxies, err := parseTrustedProxies(values.get("TRUSTED_PROXIES", ""))
	problems = appendIf(problems, err)
	cfg.TrustedProxies = proxies

	public, err := parsePublicURL(values.get("PUBLIC_URL", ""), cfg.ListenAddr)
	problems = appendIf(problems, err)
	cfg.PublicURL = public

	secretKey, err := parseSecretKey(values.get("SECRET_KEY", ""))
	problems = appendIf(problems, err)
	cfg.SecretKey = secretKey

	if cfg.DatabaseURL == "" && cfg.DataDir == "" {
		problems = append(problems, errors.New("DATA_DIR must not be empty when DATABASE_URL is unset"))
	}

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(problems...))
	}
	return cfg, nil
}

type reader struct {
	lookup Lookup
}

func (r reader) get(key, fallback string) string {
	value, ok := r.lookup(key)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func appendIf(problems []error, err error) []error {
	if err == nil {
		return problems
	}
	return append(problems, err)
}

func parseLogLevel(raw string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(raw))); err != nil {
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL %q must be debug, info, warn or error", raw)
	}
	return level, nil
}

func validateListenAddr(addr string) error {
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "" {
		return fmt.Errorf("LISTEN_ADDR %q must look like host:port or :port", addr)
	}
	return nil
}

func validateDatabaseURL(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		return errors.New("DATABASE_URL must be a postgres:// or postgresql:// URL with a host")
	}
	return nil
}

func validateSentryDSN(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("SENTRY_DSN must be a URL with a scheme and host")
	}
	return nil
}

func parseSecretKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	key, err := secrets.ParseKey(raw)
	if err != nil {
		return nil, fmt.Errorf("SECRET_KEY: %w", err)
	}
	return key, nil
}

func validateKeyPrefix(prefix string) error {
	if !keyPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("API_KEY_PREFIX %q must be 1 to 16 letters, digits or underscores", prefix)
	}
	return nil
}

func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	if raw == "" {
		return nil, nil
	}
	var prefixes []netip.Prefix
	var problems []error
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, err := parseProxyEntry(entry)
		if err != nil {
			problems = append(problems, fmt.Errorf("TRUSTED_PROXIES entry %q must be an IP address or a CIDR range", entry))
			continue
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, errors.Join(problems...)
}

func parseProxyEntry(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		return netip.ParsePrefix(entry)
	}
	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}
