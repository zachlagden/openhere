package hosting

import (
	"net"
	"regexp"
	"strings"
)

var labelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func NormaliseHost(raw string) string {
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	return host
}

func SubdomainSlug(host, baseDomain string) (string, bool) {
	if baseDomain == "" {
		return "", false
	}
	label, found := strings.CutSuffix(host, "."+baseDomain)
	if !found || !labelPattern.MatchString(label) {
		return "", false
	}
	return label, true
}

func validSlug(candidate string) bool {
	return labelPattern.MatchString(candidate)
}
