package domains

import (
	"net/netip"
	"regexp"
	"strings"

	"golang.org/x/net/idna"

	"github.com/zachlagden/selfhost/internal/validate"
)

const (
	ChallengeLabel = "_selfhost-challenge"

	maxNameLength  = 253
	maxLabelLength = 63
	minLabels      = 2
)

var maxDomainLength = maxNameLength - len(ChallengeLabel) - 1

var (
	labelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	tldPattern   = regexp.MustCompile(`^([a-z]{2,63}|xn--[a-z0-9-]{1,59})$`)
)

var reservedSuffixes = []string{
	"local", "localhost", "localdomain", "internal", "intranet", "private", "corp", "home", "lan",
	"test", "example", "invalid", "onion", "arpa", "alt", "wpad", "kubernetes", "cluster", "svc",
}

var profile = idna.New(
	idna.MapForLookup(),
	idna.BidiRule(),
	idna.StrictDomainName(true),
	idna.VerifyDNSLength(true),
	idna.CheckJoiners(true),
)

func Normalise(raw string) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", validate.Invalid("Enter a domain name such as www.example.com.")
	}
	if strings.ContainsAny(candidate, "/\\:@?#*% \t\r\n[]") {
		return "", validate.Invalid("Enter a bare domain name without a scheme, port, path or wildcard.")
	}
	candidate = strings.ToLower(candidate)
	candidate = strings.TrimSuffix(candidate, ".")
	if _, err := netip.ParseAddr(candidate); err == nil {
		return "", validate.Invalid("Enter a domain name, not an IP address.")
	}
	ascii, err := profile.ToASCII(candidate)
	if err != nil {
		return "", validate.Invalid("That is not a valid domain name.")
	}
	return checkASCII(ascii)
}

func checkASCII(name string) (string, error) {
	if len(name) > maxDomainLength {
		return "", validate.Invalid("That domain name is too long.")
	}
	labels := strings.Split(name, ".")
	if len(labels) < minLabels {
		return "", validate.Invalid("Enter a full domain name with at least two parts, such as www.example.com.")
	}
	for _, label := range labels {
		if len(label) > maxLabelLength || !labelPattern.MatchString(label) {
			return "", validate.Invalid("That is not a valid domain name.")
		}
	}
	tld := labels[len(labels)-1]
	if !tldPattern.MatchString(tld) {
		return "", validate.Invalid("The last part of the domain must be a real top-level domain.")
	}
	if isReserved(tld) || strings.HasSuffix(name, ".home.arpa") {
		return "", validate.Invalid("That domain name is reserved and cannot be used on the internet.")
	}
	return name, nil
}

func isReserved(tld string) bool {
	for _, reserved := range reservedSuffixes {
		if tld == reserved {
			return true
		}
	}
	return false
}
