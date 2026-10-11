package access

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/validate"
)

var domainLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func normalizeEmails(raw []string) ([]string, error) {
	if len(raw) > MaxAllowlistEntries {
		return nil, validate.Invalid(fmt.Sprintf("The email allowlist can hold at most %d addresses.", MaxAllowlistEntries))
	}
	clean := make([]string, 0, len(raw))
	for i, entry := range raw {
		email, err := auth.NormalizeEmail(entry)
		if err != nil {
			return nil, validate.Invalid(fmt.Sprintf("Email allowlist entry %d is not a valid email address.", i+1))
		}
		clean = append(clean, email)
	}
	return sortedUnique(clean), nil
}

func normalizeDomains(raw []string) ([]string, error) {
	if len(raw) > MaxAllowlistEntries {
		return nil, validate.Invalid(fmt.Sprintf("The domain allowlist can hold at most %d domains.", MaxAllowlistEntries))
	}
	clean := make([]string, 0, len(raw))
	for i, entry := range raw {
		domain, ok := normalizeDomain(entry)
		if !ok {
			return nil, validate.Invalid(fmt.Sprintf("Domain allowlist entry %d must be a bare domain such as example.com.", i+1))
		}
		clean = append(clean, domain)
	}
	return sortedUnique(clean), nil
}

func normalizeDomain(raw string) (string, bool) {
	domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "@")
	if domain == "" || len(domain) > maxDomainLength {
		return "", false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, label := range labels {
		if !domainLabel.MatchString(label) {
			return "", false
		}
	}
	return domain, true
}

func sortedUnique(values []string) []string {
	slices.Sort(values)
	return slices.Compact(values)
}

func domainOf(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}
