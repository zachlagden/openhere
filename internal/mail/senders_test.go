package mail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/mail/smtptest"
	"github.com/zachlagden/selfhost/internal/settings"
)

type staticSource struct {
	cfg Config
	err error
}

func (s *staticSource) MailConfig(context.Context) (Config, error) { return s.cfg, s.err }

func smtpConfig(server *smtptest.Server, security string) Config {
	return Config{
		Provider: settings.ProviderSMTP, FromAddress: "noreply@example.com", FromName: "Acme Ltd",
		SMTP: SMTPConfig{Host: server.Host(), Port: server.Port(), Security: security},
	}
}

func newSender(t *testing.T, cfg Config, server *smtptest.Server) (*Dynamic, *staticSource) {
	t.Helper()
	source := &staticSource{cfg: cfg}
	opts := Options{Timeout: 3 * time.Second}
	if server != nil {
		opts.TLSConfig = server.TLSConfig()
	}
	return New(source, opts), source
}

var sample = Message{To: "sam@example.com", Subject: "Your code", Text: "Your code is 12345678.\nIt expires soon.\n"}

func sendOne(t *testing.T, sender *Dynamic) error {
	t.Helper()
	return sender.Send(t.Context(), sample)
}

func TestSMTPSendsInEveryTransportMode(t *testing.T) {
	cases := map[string]struct {
		mode     smtptest.Mode
		security string
	}{
		"plain":    {smtptest.Plain, settings.SecurityNone},
		"implicit": {smtptest.ImplicitTLS, settings.SecurityTLS},
		"starttls": {smtptest.StartTLS, settings.SecurityStartTLS},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := smtptest.Start(t, smtptest.Options{Mode: tc.mode})
			sender, _ := newSender(t, smtpConfig(server, tc.security), server)
			if !sender.Configured() {
				t.Fatal("the sender is not configured")
			}
			if err := sendOne(t, sender); err != nil {
				t.Fatal(err)
			}
			got := server.Messages()
			if len(got) != 1 {
				t.Fatalf("messages = %d, want 1", len(got))
			}
			msg := got[0]
			if msg.From != "noreply@example.com" || len(msg.To) != 1 || msg.To[0] != "sam@example.com" {
				t.Errorf("envelope = %+v", msg)
			}
			if msg.Header("Subject") != "Your code" || !strings.Contains(msg.Header("From"), "Acme Ltd") || msg.Text() != sample.Text {
				t.Errorf("message = %q %q %q", msg.Header("Subject"), msg.Header("From"), msg.Text())
			}
		})
	}
}

func TestSMTPAuthenticatesWithPlainAndLogin(t *testing.T) {
	for _, mechanisms := range []string{"PLAIN", "LOGIN"} {
		t.Run(mechanisms, func(t *testing.T) {
			server := smtptest.Start(t, smtptest.Options{Mode: smtptest.StartTLS, Username: "alex", Password: "pw-example", AuthMechanisms: mechanisms})
			cfg := smtpConfig(server, settings.SecurityStartTLS)
			cfg.SMTP.Username, cfg.SMTP.Password = "alex", "pw-example"
			sender, _ := newSender(t, cfg, server)
			if err := sendOne(t, sender); err != nil {
				t.Fatal(err)
			}
			if got := server.Messages(); len(got) != 1 || got[0].AuthUser != "alex" {
				t.Fatalf("messages = %+v", got)
			}
		})
	}
}

func TestSMTPAuthFailureIsSanitised(t *testing.T) {
	server := smtptest.Start(t, smtptest.Options{Mode: smtptest.Plain, Username: "alex", Password: "right-password"})
	cfg := smtpConfig(server, settings.SecurityNone)
	cfg.SMTP.Username, cfg.SMTP.Password = "alex", "wrong-password"
	sender, _ := newSender(t, cfg, server)
	err := sendOne(t, sender)
	if err == nil {
		t.Fatal("a wrong password was accepted")
	}
	reason := Reason(err)
	if !strings.Contains(reason, "username or password") || strings.Contains(reason, "wrong-password") || strings.Contains(reason, "5.7.8") {
		t.Errorf("reason = %q", reason)
	}
	if len(server.Messages()) != 0 {
		t.Error("a message was delivered without authentication")
	}
}

func TestSMTPTimesOutOnAStalledServer(t *testing.T) {
	server := smtptest.Start(t, smtptest.Options{Stall: true})
	source := &staticSource{cfg: smtpConfig(server, settings.SecurityNone)}
	sender := New(source, Options{Timeout: 300 * time.Millisecond})
	started := time.Now()
	err := sendOne(t, sender)
	if err == nil || !strings.Contains(Reason(err), "timed out") {
		t.Fatalf("error = %v, want a timeout", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Error("the timeout took far too long")
	}
}

func TestSMTPFailureReasons(t *testing.T) {
	t.Run("untrusted certificate", func(t *testing.T) {
		server := smtptest.Start(t, smtptest.Options{Mode: smtptest.ImplicitTLS})
		sender := New(&staticSource{cfg: smtpConfig(server, settings.SecurityTLS)}, Options{Timeout: 3 * time.Second})
		if err := sendOne(t, sender); err == nil || !strings.Contains(Reason(err), "TLS handshake") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("starttls not offered", func(t *testing.T) {
		server := smtptest.Start(t, smtptest.Options{Mode: smtptest.Plain})
		sender, _ := newSender(t, smtpConfig(server, settings.SecurityStartTLS), server)
		if err := sendOne(t, sender); err == nil || !strings.Contains(Reason(err), "STARTTLS") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("recipient rejected", func(t *testing.T) {
		server := smtptest.Start(t, smtptest.Options{RejectRcpt: true})
		sender, _ := newSender(t, smtpConfig(server, settings.SecurityNone), server)
		err := sendOne(t, sender)
		if err == nil || !strings.Contains(Reason(err), "recipient") || !strings.Contains(Reason(err), "550") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("connection refused", func(t *testing.T) {
		server := smtptest.Start(t, smtptest.Options{})
		cfg := smtpConfig(server, settings.SecurityNone)
		server.Close()
		sender, _ := newSender(t, cfg, nil)
		if err := sendOne(t, sender); err == nil || !strings.Contains(Reason(err), "connecting") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("header injection", func(t *testing.T) {
		server := smtptest.Start(t, smtptest.Options{})
		sender, _ := newSender(t, smtpConfig(server, settings.SecurityNone), server)
		bad := Message{To: "sam@example.com", Subject: "Hi\r\nBcc: eve@example.com", Text: "x"}
		if err := sender.Send(t.Context(), bad); err == nil {
			t.Fatal("a subject with a line break was sent")
		}
		if len(server.Messages()) != 0 {
			t.Error("an injected message reached the server")
		}
	})
}

func resendServer(t *testing.T, status int, reply string) (*httptest.Server, *[]resendCall) {
	t.Helper()
	calls := &[]resendCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*calls = append(*calls, resendCall{auth: r.Header.Get("Authorization"), body: body, method: r.Method})
		w.WriteHeader(status)
		if _, err := io.WriteString(w, reply); err != nil {
			t.Errorf("write reply: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server, calls
}

type resendCall struct {
	auth, method string
	body         []byte
}

func resendConfig() Config {
	return Config{Provider: settings.ProviderResend, FromAddress: "noreply@example.com", FromName: "Acme Ltd", ResendAPIKey: "re_example_key"}
}

func TestResendSendsTheMessage(t *testing.T) {
	server, calls := resendServer(t, http.StatusOK, `{"id":"abc"}`)
	sender := New(&staticSource{cfg: resendConfig()}, Options{ResendEndpoint: server.URL, Timeout: 3 * time.Second})
	if !sender.Configured() {
		t.Fatal("the sender is not configured")
	}
	if err := sendOne(t, sender); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0].method != "POST" || (*calls)[0].auth != "Bearer re_example_key" {
		t.Fatalf("calls = %+v", *calls)
	}
	var payload resendPayload
	if err := json.Unmarshal((*calls)[0].body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Subject != "Your code" || payload.Text != sample.Text || len(payload.To) != 1 || payload.To[0] != "sam@example.com" || !strings.Contains(payload.From, "noreply@example.com") {
		t.Errorf("payload = %+v", payload)
	}
}

func TestResendErrorsAreSanitised(t *testing.T) {
	server, _ := resendServer(t, http.StatusForbidden, `{"name":"validation_error","message":"The re_example_key domain is not verified"}`)
	sender := New(&staticSource{cfg: resendConfig()}, Options{ResendEndpoint: server.URL, Timeout: 3 * time.Second})
	err := sendOne(t, sender)
	if err == nil {
		t.Fatal("a 403 was treated as success")
	}
	reason := Reason(err)
	if reason != "Resend rejected the request (status 403, validation_error)" || strings.Contains(err.Error(), "re_example_key") {
		t.Errorf("reason = %q", reason)
	}
	odd, _ := resendServer(t, http.StatusBadGateway, `<html>Server banner v9.9</html>`)
	sender = New(&staticSource{cfg: resendConfig()}, Options{ResendEndpoint: odd.URL, Timeout: 3 * time.Second})
	if got := Reason(sendOne(t, sender)); got != "Resend rejected the request (status 502)" {
		t.Errorf("reason = %q", got)
	}
}

func TestResendTimesOut(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	sender := New(&staticSource{cfg: resendConfig()}, Options{ResendEndpoint: slow.URL, Timeout: 200 * time.Millisecond})
	if err := sendOne(t, sender); err == nil || !strings.Contains(Reason(err), "timed out") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfiguredStates(t *testing.T) {
	good := Config{Provider: settings.ProviderSMTP, FromAddress: "noreply@example.com", SMTP: SMTPConfig{Host: "smtp.example.com", Port: 587, Security: settings.SecurityStartTLS}}
	cases := map[string]struct {
		change func(*Config)
		want   bool
	}{
		"complete smtp":        {func(*Config) {}, true},
		"smtp with login":      {func(c *Config) { c.SMTP.Username, c.SMTP.Password = "alex", "pw" }, true},
		"provider none":        {func(c *Config) { c.Provider = settings.ProviderNone }, false},
		"no provider":          {func(c *Config) { c.Provider = "" }, false},
		"no from address":      {func(c *Config) { c.FromAddress = "" }, false},
		"no host":              {func(c *Config) { c.SMTP.Host = "" }, false},
		"port zero":            {func(c *Config) { c.SMTP.Port = 0 }, false},
		"port too high":        {func(c *Config) { c.SMTP.Port = 70000 }, false},
		"bad security":         {func(c *Config) { c.SMTP.Security = "ssl" }, false},
		"username no password": {func(c *Config) { c.SMTP.Username = "alex" }, false},
		"resend complete":      {func(c *Config) { c.Provider, c.ResendAPIKey = settings.ProviderResend, "re_key" }, true},
		"resend no key":        {func(c *Config) { c.Provider = settings.ProviderResend }, false},
	}
	for name, tc := range cases {
		cfg := good
		tc.change(&cfg)
		if got := cfg.Configured(); got != tc.want {
			t.Errorf("%s: Configured = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSettingsChangeTakeEffectWithoutRestart(t *testing.T) {
	first := smtptest.Start(t, smtptest.Options{})
	second := smtptest.Start(t, smtptest.Options{})
	sender, source := newSender(t, smtpConfig(first, settings.SecurityNone), first)
	if err := sendOne(t, sender); err != nil {
		t.Fatal(err)
	}
	source.cfg = smtpConfig(second, settings.SecurityNone)
	if err := sendOne(t, sender); err != nil {
		t.Fatal(err)
	}
	if len(first.Messages()) != 1 || len(second.Messages()) != 1 {
		t.Fatalf("messages = %d and %d, want 1 and 1", len(first.Messages()), len(second.Messages()))
	}
	source.cfg.Provider = settings.ProviderNone
	if sender.Configured() {
		t.Error("the sender stayed configured after the provider was switched off")
	}
	if err := sendOne(t, sender); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Send error = %v, want ErrNotConfigured", err)
	}
}

func TestUnreadableSettingsMakeTheSenderUnconfigured(t *testing.T) {
	source := &staticSource{err: errors.New("cannot open the password")}
	sender := New(source, Options{})
	if sender.Configured() {
		t.Error("the sender is configured although the settings cannot be read")
	}
	if err := sendOne(t, sender); err == nil || !strings.Contains(Reason(err), "cannot be read") {
		t.Errorf("error = %v", err)
	}
}

func TestSMTPRefusesPasswordsOverAnUnencryptedConnection(t *testing.T) {
	cfg := Config{
		Provider: settings.ProviderSMTP, FromAddress: "noreply@example.com",
		SMTP: SMTPConfig{Host: "mail.example.com", Port: 25, Security: settings.SecurityNone, Username: "alex", Password: "a-password"},
	}
	sender := New(&staticSource{cfg: cfg}, Options{Timeout: 3 * time.Second})
	err := sendOne(t, sender)
	if err == nil {
		t.Fatal("credentials were sent over an unencrypted connection")
	}
	reason := Reason(err)
	if !strings.Contains(reason, "not encrypted") || strings.Contains(reason, "a-password") || strings.Contains(reason, "connection failed") {
		t.Errorf("reason = %q, want the refusal and no attempt to connect", reason)
	}
}

func TestEncryptedAuthRule(t *testing.T) {
	cases := map[string]struct {
		cfg     SMTPConfig
		refused bool
	}{
		"remote none with login":     {SMTPConfig{Host: "mail.example.com", Security: settings.SecurityNone, Username: "alex"}, true},
		"remote none with ip":        {SMTPConfig{Host: "203.0.113.5", Security: settings.SecurityNone, Username: "alex"}, true},
		"remote none without login":  {SMTPConfig{Host: "mail.example.com", Security: settings.SecurityNone}, false},
		"remote starttls with login": {SMTPConfig{Host: "mail.example.com", Security: settings.SecurityStartTLS, Username: "alex"}, false},
		"remote tls with login":      {SMTPConfig{Host: "mail.example.com", Security: settings.SecurityTLS, Username: "alex"}, false},
		"localhost none with login":  {SMTPConfig{Host: "localhost", Security: settings.SecurityNone, Username: "alex"}, false},
		"loopback v4 none":           {SMTPConfig{Host: "127.0.0.1", Security: settings.SecurityNone, Username: "alex"}, false},
		"loopback v6 none":           {SMTPConfig{Host: "::1", Security: settings.SecurityNone, Username: "alex"}, false},
		"lookalike host":             {SMTPConfig{Host: "localhost.example.com", Security: settings.SecurityNone, Username: "alex"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := requireEncryptedAuth(tc.cfg) != nil; got != tc.refused {
				t.Errorf("refused = %v, want %v", got, tc.refused)
			}
		})
	}
}
