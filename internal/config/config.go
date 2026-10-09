package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
)

const (
	defaultListenAddr = ":8080"
	defaultDataDir    = "./data"
	defaultLogLevel   = "info"
)

type Config struct {
	ListenAddr  string
	DataDir     string
	DatabaseURL string
	LogLevel    slog.Level
	SentryDSN   string
}

type Lookup func(key string) (string, bool)

func Load(lookup Lookup) (Config, error) {
	values := reader{lookup: lookup}

	cfg := Config{
		ListenAddr:  values.get("LISTEN_ADDR", defaultListenAddr),
		DataDir:     values.get("DATA_DIR", defaultDataDir),
		DatabaseURL: values.get("DATABASE_URL", ""),
		SentryDSN:   values.get("SENTRY_DSN", ""),
	}

	var problems []error

	level, err := parseLogLevel(values.get("LOG_LEVEL", defaultLogLevel))
	problems = appendIf(problems, err)
	cfg.LogLevel = level

	problems = appendIf(problems, validateListenAddr(cfg.ListenAddr))
	problems = appendIf(problems, validateDatabaseURL(cfg.DatabaseURL))
	problems = appendIf(problems, validateSentryDSN(cfg.SentryDSN))

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
