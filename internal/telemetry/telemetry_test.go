package telemetry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewLoggerWritesJSONAtLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, slog.LevelWarn)

	logger.Info("hidden")
	logger.Warn("shown", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("output is not a single JSON object: %v: %q", err, buf.String())
	}
	if entry["msg"] != "shown" || entry["key"] != "value" {
		t.Errorf("unexpected entry: %v", entry)
	}
}

func TestInitSentryDisabledWithoutDSN(t *testing.T) {
	flush, enabled, err := InitSentry("", "test")
	if err != nil {
		t.Fatalf("InitSentry returned error: %v", err)
	}
	if enabled {
		t.Error("sentry should stay disabled without a DSN")
	}
	flush()
}

func TestInitSentryRejectsBadDSN(t *testing.T) {
	if _, _, err := InitSentry("https://", "test"); err == nil {
		t.Error("expected an error for an invalid DSN")
	}
}
