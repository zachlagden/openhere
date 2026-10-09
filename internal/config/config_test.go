package config

import (
	"log/slog"
	"strings"
	"testing"
)

func lookupFrom(values map[string]string) Lookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != ":8080" || cfg.DataDir != "./data" || cfg.DatabaseURL != "" || cfg.SentryDSN != "" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("log level = %v, want info", cfg.LogLevel)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{
		"LISTEN_ADDR":  "127.0.0.1:9000",
		"DATA_DIR":     "/srv/data",
		"DATABASE_URL": "postgres://db.example.com:5432/app",
		"LOG_LEVEL":    "DEBUG",
		"SENTRY_DSN":   "https://key@glitchtip.example.com/1",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:9000" || cfg.DataDir != "/srv/data" {
		t.Errorf("unexpected values: %+v", cfg)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("log level = %v, want debug", cfg.LogLevel)
	}
	if cfg.DatabaseURL == "" || cfg.SentryDSN == "" {
		t.Errorf("database URL and sentry DSN should be set: %+v", cfg)
	}
}

func TestLoadBlankValuesUseDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{"LISTEN_ADDR": "  ", "LOG_LEVEL": ""}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != ":8080" || cfg.LogLevel != slog.LevelInfo {
		t.Errorf("blank values should fall back to defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		want string
	}{
		"listen address without port": {map[string]string{"LISTEN_ADDR": "localhost"}, "LISTEN_ADDR"},
		"unknown log level":           {map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		"non postgres database url":   {map[string]string{"DATABASE_URL": "mysql://db.example.com/app"}, "DATABASE_URL"},
		"database url without host":   {map[string]string{"DATABASE_URL": "postgres:///app"}, "DATABASE_URL"},
		"sentry dsn not a url":        {map[string]string{"SENTRY_DSN": "not-a-dsn"}, "SENTRY_DSN"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(lookupFrom(tc.env))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should mention %s", err, tc.want)
			}
		})
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, err := Load(lookupFrom(map[string]string{"LISTEN_ADDR": "nope", "LOG_LEVEL": "loud"}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"LISTEN_ADDR", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q should mention %s", err, key)
		}
	}
}
