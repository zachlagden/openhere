package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/mail/smtptest"
	"github.com/zachlagden/openhere/internal/secrets"
	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func boxWith(t *testing.T, fill byte) *secrets.Box {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{fill}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func patchOf(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSettingsSourceDecryptsSecretsAndSendsLive(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		server := smtptest.Start(t, smtptest.Options{Username: "alex", Password: "pw-example-secret"})
		box := boxWith(t, 4)
		svc := settings.New(db, settings.Options{Secrets: box})
		logs := &bytes.Buffer{}
		logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		sender := New(NewSettingsSource(svc, box), Options{Logger: logger, Timeout: 3 * time.Second})
		if sender.Configured() {
			t.Fatal("a fresh instance reports email as configured")
		}
		body, err := json.Marshal(map[string]any{
			"email.provider": "smtp", "email.fromAddress": "noreply@example.com", "email.smtp.host": server.Host(),
			"email.smtp.port": server.Port(), "email.smtp.security": "none", "email.smtp.username": "alex",
			"email.smtp.password": "pw-example-secret",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Update(context.Background(), patchOf(t, string(body))); err != nil {
			t.Fatal(err)
		}
		if !sender.Configured() {
			t.Fatal("the sender ignored the new settings")
		}
		if err := sender.Send(context.Background(), sample); err != nil {
			t.Fatal(err)
		}
		if got := server.Messages(); len(got) != 1 || got[0].AuthUser != "alex" {
			t.Fatalf("messages = %+v", got)
		}
		if _, err := svc.Update(context.Background(), patchOf(t, `{"email.smtp.password": "changed-password"}`)); err != nil {
			t.Fatal(err)
		}
		if err := sender.Send(context.Background(), sample); err == nil {
			t.Fatal("the old password still worked after the change")
		}
		if strings.Contains(logs.String(), "pw-example-secret") || strings.Contains(logs.String(), "changed-password") {
			t.Errorf("a secret reached the logs: %s", logs.String())
		}
	})
}

func TestSettingsSourceRejectsAWrongKey(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := settings.New(db, settings.Options{Secrets: boxWith(t, 1)})
		if _, err := svc.Update(context.Background(), patchOf(t, `{"email.provider": "resend", "email.fromAddress": "noreply@example.com", "email.resend.apiKey": "re_key"}`)); err != nil {
			t.Fatal(err)
		}
		sender := New(NewSettingsSource(svc, boxWith(t, 2)), Options{})
		if sender.Configured() {
			t.Error("a sender with the wrong secret key reports itself as configured")
		}
		if err := sender.Send(context.Background(), sample); err == nil {
			t.Error("a sender with the wrong secret key sent mail")
		}
	})
}
