package certs

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestCertmagicLogsGoThroughSlogWithoutAttackerNames(t *testing.T) {
	var out bytes.Buffer
	logger := newCertmagicLogger(slog.New(slog.NewTextHandler(&out, nil)))
	logger.Debug("debug detail", zap.String("identifier", "hidden.example.com"))
	logger.Error("tls-alpn challenge", zap.String("server_name", "attacker.example.net"))
	logger.Info("obtaining certificate", zap.String("identifier", "www.example.org"))
	logger.Named("obtain").With(zap.String("issuer", "ca")).Warn("slow", zap.Int("attempt", 2))

	text := out.String()
	if strings.Contains(text, "attacker") || strings.Contains(text, "hidden") {
		t.Errorf("unexpected content in the log: %s", text)
	}
	for _, want := range []string{"level=INFO", "certmagic: obtaining certificate", "identifier=www.example.org", "level=WARN", "component=obtain", "issuer=ca", "attempt=2"} {
		if !strings.Contains(text, want) {
			t.Errorf("log lacks %q: %s", want, text)
		}
	}
}
