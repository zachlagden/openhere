package analytics

import (
	"net"
	"net/url"
	"strings"
)

const maxReferrerHostLength = 100

func ReferrerHost(rawReferrer, ownHost string) string {
	trimmed := strings.TrimSpace(rawReferrer)
	if trimmed == "" || !strings.Contains(trimmed, "://") {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "" || len(host) > maxReferrerHostLength || hasControlOrSpace(host) {
		return ""
	}
	if host == strings.TrimPrefix(strings.ToLower(hostOnly(ownHost)), "www.") {
		return ""
	}
	return host
}

func hostOnly(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}

func hasControlOrSpace(value string) bool {
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
