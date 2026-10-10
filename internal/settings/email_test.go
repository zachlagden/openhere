package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/secrets"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func testBox(t *testing.T) *secrets.Box {
	t.Helper()
	box, err := secrets.New(bytes.Repeat([]byte{5}, secrets.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestEmailSettingsRoundTrip(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{Secrets: testBox(t)})
		updated, err := svc.Update(ctx, patch(t, `{
			"email.provider": "smtp", "email.fromAddress": " Noreply@Example.com ", "email.fromName": "Acme Ltd",
			"email.smtp.host": "SMTP.Example.com", "email.smtp.port": 465, "email.smtp.security": "tls",
			"email.smtp.username": "alex", "email.smtp.password": "hunter2-example"}`))
		if err != nil {
			t.Fatal(err)
		}
		want := Email{
			Provider: ProviderSMTP, FromAddress: "noreply@example.com", FromName: "Acme Ltd", SMTPHost: "smtp.example.com",
			SMTPPort: 465, SMTPSecurity: SecurityTLS, SMTPUsername: "alex",
		}
		got := updated.Email
		sealed := got.SMTPPassword
		got.SMTPPassword = ""
		if got != want {
			t.Errorf("email = %+v, want %+v", got, want)
		}
		if sealed == "" || strings.Contains(sealed, "hunter2-example") {
			t.Errorf("the SMTP password is not sealed: %q", sealed)
		}
		var stored string
		if err := db.QueryRowContext(ctx, "SELECT value FROM instance_settings WHERE key = $1", KeySMTPPassword).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stored, "hunter2-example") || stored != sealed {
			t.Errorf("the stored password is %q", stored)
		}
		plain, err := testBox(t).Open(sealed)
		if err != nil || plain != "hunter2-example" {
			t.Errorf("Open = %q, %v", plain, err)
		}
	})
}

func TestSecretsNeverAppearInValues(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{Secrets: testBox(t)})
		updated, err := svc.Update(context.Background(), patch(t, `{"email.smtp.password": "hunter2-example", "email.resend.apiKey": "re_example_key"}`))
		if err != nil {
			t.Fatal(err)
		}
		values := Values(updated)
		encoded, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "hunter2-example") || strings.Contains(string(encoded), "re_example_key") {
			t.Fatalf("a secret leaked into the values: %s", encoded)
		}
		if values["email.smtp.passwordSet"] != true || values["email.resend.apiKeySet"] != true {
			t.Errorf("secret flags = %v", values)
		}
		if _, leaked := values[KeySMTPPassword]; leaked {
			t.Error("the secret key itself is exposed")
		}
	})
}

func TestSecretsCanBeClearedAndNeedAKey(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		keyless := New(db, Options{})
		if _, err := keyless.Update(ctx, patch(t, `{"email.smtp.password": "x"}`)); err == nil {
			t.Fatal("a secret was stored without a key")
		}
		svc := New(db, Options{Secrets: testBox(t)})
		if _, err := svc.Update(ctx, patch(t, `{"email.smtp.password": "x"}`)); err != nil {
			t.Fatal(err)
		}
		cleared, err := keyless.Update(ctx, patch(t, `{"email.smtp.password": ""}`))
		if err != nil {
			t.Fatal(err)
		}
		if cleared.Email.SMTPPassword != "" {
			t.Error("the secret survived clearing")
		}
	})
}

func TestEmailSettingsRejectBadInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{Secrets: testBox(t)})
		for name, body := range map[string]string{
			"provider":      `{"email.provider": "sendgrid"}`,
			"security":      `{"email.smtp.security": "ssl"}`,
			"port zero":     `{"email.smtp.port": 0}`,
			"port high":     `{"email.smtp.port": 70000}`,
			"host scheme":   `{"email.smtp.host": "smtp://mail.example.com"}`,
			"host port":     `{"email.smtp.host": "mail.example.com:25"}`,
			"from address":  `{"email.fromAddress": "Alex <alex@example.com>"}`,
			"from no tld":   `{"email.fromAddress": "alex@localhost"}`,
			"from name":     `{"email.fromName": "Acme\r\nBcc: x@example.com"}`,
			"username":      `{"email.smtp.username": "a\nb"}`,
			"secret type":   `{"email.smtp.password": 5}`,
			"secret null":   `{"email.resend.apiKey": null}`,
			"invite policy": `{"invites.whoCanInvite": "everyone"}`,
			"create policy": `{"workspaces.whoCanCreate": "nobody"}`,
			"secret set":    `{"email.smtp.passwordSet": true}`,
		} {
			if _, err := svc.Update(context.Background(), patch(t, body)); !isValidation(err) {
				t.Errorf("%s: error = %v, want a validation error", name, err)
			}
		}
	})
}

func TestPolicySettingsAcceptTheirChoices(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		updated, err := svc.Update(context.Background(), patch(t, `{"invites.whoCanInvite": "workspaceAdmins", "workspaces.whoCanCreate": "root", "email.smtp.host": "192.0.2.10"}`))
		if err != nil {
			t.Fatal(err)
		}
		if updated.WhoCanInvite != InviteWorkspaceAdmins || updated.WhoCanCreate != CreateRoot || updated.Email.SMTPHost != "192.0.2.10" {
			t.Errorf("updated = %+v", updated)
		}
	})
}
