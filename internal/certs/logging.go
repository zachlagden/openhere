package certs

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var droppedMessages = map[string]struct{}{
	"tls-alpn challenge":                     {},
	"TLS handshake aborted by event handler": {},
}

type slogCore struct {
	logger *slog.Logger
	fields []zapcore.Field
}

func newCertmagicLogger(logger *slog.Logger) *zap.Logger {
	return zap.New(&slogCore{logger: logger})
}

func (c *slogCore) Enabled(level zapcore.Level) bool {
	return level >= zapcore.InfoLevel
}

func (c *slogCore) With(fields []zapcore.Field) zapcore.Core {
	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)
	return &slogCore{logger: c.logger, fields: merged}
}

func (c *slogCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(entry.Level) {
		return checked
	}
	if _, dropped := droppedMessages[entry.Message]; dropped {
		return checked
	}
	return checked.AddCore(entry, c)
}

func (c *slogCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range c.fields {
		field.AddTo(encoder)
	}
	for _, field := range fields {
		field.AddTo(encoder)
	}
	attrs := make([]slog.Attr, 0, len(encoder.Fields)+1)
	if entry.LoggerName != "" {
		attrs = append(attrs, slog.String("component", entry.LoggerName))
	}
	for key, value := range encoder.Fields {
		attrs = append(attrs, slog.Any(key, value))
	}
	c.logger.LogAttrs(context.Background(), slogLevel(entry.Level), "certmagic: "+entry.Message, attrs...)
	return nil
}

func (c *slogCore) Sync() error {
	return nil
}

func slogLevel(level zapcore.Level) slog.Level {
	switch {
	case level >= zapcore.ErrorLevel:
		return slog.LevelError
	case level == zapcore.WarnLevel:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
