package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const defaultPublicHost = "localhost"

type PublicURL struct {
	Scheme string
	Host   string
	Port   string
}

func (p PublicURL) Secure() bool {
	return p.Scheme == "https"
}

func (p PublicURL) Authority() string {
	host := p.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if p.Port == "" {
		return host
	}
	return host + ":" + p.Port
}

func (p PublicURL) String() string {
	return p.Scheme + "://" + p.Authority()
}

func parsePublicURL(raw, listenAddr string) (PublicURL, error) {
	if raw == "" {
		return defaultPublicURL(listenAddr), nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return PublicURL{}, publicURLProblem(raw)
	}
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	validShape := (scheme == "http" || scheme == "https") && host != "" && parsed.User == nil &&
		(parsed.Path == "" || parsed.Path == "/") && parsed.RawQuery == "" && parsed.Fragment == "" && !parsed.ForceQuery
	if !validShape {
		return PublicURL{}, publicURLProblem(raw)
	}
	port := parsed.Port()
	if port != "" {
		if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
			return PublicURL{}, publicURLProblem(raw)
		}
	}
	return PublicURL{Scheme: scheme, Host: host, Port: port}, nil
}

func publicURLProblem(raw string) error {
	return fmt.Errorf("PUBLIC_URL %q must be an http:// or https:// URL with a host, such as https://sites.example.com, and no path, query or credentials", raw)
}

func defaultPublicURL(listenAddr string) PublicURL {
	public := PublicURL{Scheme: "http", Host: defaultPublicHost}
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil || port == "" || port == "80" {
		return public
	}
	public.Port = port
	return public
}
