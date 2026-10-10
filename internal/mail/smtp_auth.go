package mail

import (
	"errors"
	"net/netip"
	"net/smtp"
	"strings"

	"github.com/zachlagden/selfhost/internal/settings"
)

type plainAuth struct{ username, password string }

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected server challenge")
	}
	return nil, nil
}

type loginAuth struct{ username, password string }

func (a loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a loginAuth) Next(challenge []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(challenge))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, errors.New("unexpected server challenge")
	}
}

func chooseAuth(advertised, username, password string) (smtp.Auth, bool) {
	mechanisms := strings.Fields(strings.ToUpper(advertised))
	has := func(name string) bool {
		for _, m := range mechanisms {
			if m == name {
				return true
			}
		}
		return false
	}
	switch {
	case has("PLAIN"):
		return plainAuth{username, password}, true
	case has("LOGIN"):
		return loginAuth{username, password}, true
	default:
		return nil, false
	}
}

func requireEncryptedAuth(cfg SMTPConfig) error {
	if cfg.Username == "" || cfg.Security != settings.SecurityNone || isLoopbackHost(cfg.Host) {
		return nil
	}
	return sendError("the connection is not encrypted, so the server would receive the password in clear text; choose TLS or STARTTLS", nil)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil && address.IsLoopback()
}
