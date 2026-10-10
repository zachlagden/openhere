package certs

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	httpsDefaultPort = "443"
	maxHostLength    = 253
)

func redirectHandler(httpsPort string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, ok := redirectHost(r.Host)
		if !ok {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		target := url.URL{
			Scheme: "https", Host: authority(host, httpsPort),
			Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery,
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
	})
}

func authority(host, httpsPort string) string {
	if httpsPort != "" && httpsPort != httpsDefaultPort {
		return net.JoinHostPort(host, httpsPort)
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func redirectHost(raw string) (string, bool) {
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	if bracketed := strings.HasPrefix(host, "["); bracketed {
		inner := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		return inner, strings.HasSuffix(host, "]") && strings.Contains(inner, ":") && net.ParseIP(inner) != nil
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" || len(host) > maxHostLength {
		return "", false
	}
	if strings.Contains(host, ":") {
		return host, net.ParseIP(host) != nil
	}
	for _, c := range host {
		valid := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_'
		if !valid {
			return "", false
		}
	}
	return host, true
}
