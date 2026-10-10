package config

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultTLSHTTPAddr  = ":80"
	defaultTLSHTTPSAddr = ":443"
	defaultACMECA       = "https://acme-v02.api.letsencrypt.org/directory"
	defaultTLSDirName   = "certs"
)

type TLS struct {
	Enabled   bool
	HTTPAddr  string
	HTTPSAddr string
	Email     string
	CA        string
	CARoot    string
	Storage   string
}

func (t TLS) HTTPSPort() string {
	return portOf(t.HTTPSAddr)
}

func (t TLS) HTTPPort() string {
	return portOf(t.HTTPAddr)
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

func parseTLS(values reader, dataDir, rawPublicURL string, public PublicURL) (TLS, error) {
	enabled, err := parseTLSEnabled(values.get("TLS_ENABLED", "false"))
	if err != nil {
		return TLS{}, err
	}
	if !enabled {
		return TLS{}, nil
	}
	tls := TLS{
		Enabled:   true,
		HTTPAddr:  values.get("TLS_HTTP_ADDR", defaultTLSHTTPAddr),
		HTTPSAddr: values.get("TLS_HTTPS_ADDR", defaultTLSHTTPSAddr),
		Email:     values.get("ACME_EMAIL", ""),
		CA:        values.get("ACME_CA", defaultACMECA),
		CARoot:    values.get("ACME_CA_ROOT", ""),
		Storage:   values.get("TLS_STORAGE", filepath.Join(dataDir, defaultTLSDirName)),
	}
	var problems []error
	problems = appendIf(problems, validateTLSAddr("TLS_HTTP_ADDR", tls.HTTPAddr))
	problems = appendIf(problems, validateTLSAddr("TLS_HTTPS_ADDR", tls.HTTPSAddr))
	if tls.HTTPPort() != "" && tls.HTTPPort() == tls.HTTPSPort() {
		problems = append(problems, errors.New("TLS_HTTP_ADDR and TLS_HTTPS_ADDR must use different ports"))
	}
	problems = appendIf(problems, validateACMEEmail(tls.Email))
	problems = appendIf(problems, validateACMECA(tls.CA))
	if rawPublicURL == "" || !public.Secure() {
		problems = append(problems, errors.New("PUBLIC_URL must be set to an https:// URL when TLS_ENABLED is true"))
	}
	return tls, errors.Join(problems...)
}

func parseTLSEnabled(raw string) (bool, error) {
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("TLS_ENABLED %q must be true or false", raw)
	}
	return enabled, nil
}

func validateTLSAddr(name, addr string) error {
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "" {
		return fmt.Errorf("%s %q must look like host:port or :port", name, addr)
	}
	return nil
}

func validateACMEEmail(raw string) error {
	if raw == "" {
		return errors.New("ACME_EMAIL is required when TLS_ENABLED is true")
	}
	parsed, err := mail.ParseAddress(raw)
	if err != nil || parsed.Address != raw || parsed.Name != "" {
		return errors.New("ACME_EMAIL must be a plain email address such as admin@example.com")
	}
	return nil
}

func validateACMECA(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || strings.TrimSpace(raw) != raw {
		return errors.New("ACME_CA must be the http:// or https:// URL of an ACME directory")
	}
	return nil
}
