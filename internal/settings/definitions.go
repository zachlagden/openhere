package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zachlagden/openhere/internal/validate"
)

const (
	KeyKeepVersions     = "retention.keepVersions"
	KeyMaxFilesPerVer   = "limits.maxFilesPerVersion"
	KeyMaxVersionBytes  = "limits.maxVersionBytes"
	KeyMaxFileBytes     = "limits.maxFileBytes"
	KeyMaxPendingBytes  = "limits.maxPendingBytes"
	KeyMaxGateMails     = "limits.maxGateCodeMailsPerHour"
	KeyBaseDomain       = "hosting.baseDomain"
	KeySubdomainEnabled = "hosting.subdomainEnabled"
	KeyPathEnabled      = "hosting.pathEnabled"
	KeyCustomDomains    = "hosting.customDomainsEnabled"
	KeyMaxDomains       = "hosting.maxDomainsPerSite"
	KeyWildcardStatus   = "hosting.wildcardStatus"
	KeyWildcardChecked  = "hosting.wildcardCheckedAt"
	KeyWildcardDetail   = "hosting.wildcardDetail"
	KeyWildcardFailures = "hosting.wildcardFailures"
	KeyDisabledReason   = "hosting.subdomainDisabledReason"
	KeyDefaultAccess    = "sites.defaultAccessMode"
	KeyWhoCanInvite     = "invites.whoCanInvite"
	KeyWhoCanCreate     = "workspaces.whoCanCreate"

	WildcardUnchecked = "unchecked"
	WildcardOK        = "ok"
	WildcardFailed    = "failed"

	DisabledByWildcard = "wildcard_check_failed"

	AccessPublic     = "public"
	AccessRestricted = "restricted"

	InviteRoot            = "root"
	InviteWorkspaceAdmins = "workspaceAdmins"
	CreateAny             = "any"
	CreateRoot            = "root"

	minimumInt          = 1
	maxBaseDomainLength = 253
	maxKeepVersions     = 10000
	maxDomainsPerSite   = 50
	maxFilesLimit       = 1_000_000
	maxBytesLimit       = int64(1) << 50
)

var hostnameLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type Settings struct {
	KeepVersions     int
	MaxFilesPerVer   int
	MaxVersionBytes  int64
	MaxFileBytes     int64
	MaxPendingBytes  int64
	MaxGateMails     int
	BaseDomain       string
	SubdomainEnabled bool
	PathEnabled      bool

	CustomDomainsEnabled bool
	MaxDomainsPerSite    int

	WildcardStatus    string
	WildcardCheckedAt string
	WildcardDetail    string
	WildcardFailures  int

	SubdomainDisabledReason string

	DefaultAccessMode string

	Email Email

	TLS TLS

	Analytics Analytics

	WhoCanInvite string
	WhoCanCreate string
}

type definition struct {
	key      string
	fallback string
	decode   func(raw json.RawMessage) (string, error)
	apply    func(s *Settings, stored string) error
	value    func(s Settings) any
	readOnly bool
	secret   bool
	outKey   string
}

func (d definition) outputKey() string {
	if d.outKey != "" {
		return d.outKey
	}
	return d.key
}

var definitions = []definition{
	intDefinition(KeyKeepVersions, 10, maxKeepVersions,
		func(s *Settings, v int64) { s.KeepVersions = int(v) },
		func(s Settings) int64 { return int64(s.KeepVersions) }),
	intDefinition(KeyMaxFilesPerVer, 2500, maxFilesLimit,
		func(s *Settings, v int64) { s.MaxFilesPerVer = int(v) },
		func(s Settings) int64 { return int64(s.MaxFilesPerVer) }),
	intDefinition(KeyMaxVersionBytes, 1<<30, maxBytesLimit,
		func(s *Settings, v int64) { s.MaxVersionBytes = v },
		func(s Settings) int64 { return s.MaxVersionBytes }),
	intDefinition(KeyMaxFileBytes, 256<<20, maxBytesLimit,
		func(s *Settings, v int64) { s.MaxFileBytes = v },
		func(s Settings) int64 { return s.MaxFileBytes }),
	boundedIntDefinition(KeyMaxPendingBytes, 4<<30, true, minimumInt, maxBytesLimit,
		func(s *Settings, v int64) { s.MaxPendingBytes = v },
		func(s Settings) int64 { return s.MaxPendingBytes }),
	boundedIntDefinition(KeyMaxGateMails, 100, true, minimumInt, maxFilesLimit,
		func(s *Settings, v int64) { s.MaxGateMails = int(v) },
		func(s Settings) int64 { return int64(s.MaxGateMails) }),
	stringDefinition(KeyBaseDomain, "", normaliseBaseDomain,
		func(s *Settings, v string) { s.BaseDomain = v },
		func(s Settings) string { return s.BaseDomain }),
	boolDefinition(KeySubdomainEnabled, false,
		func(s *Settings, v bool) { s.SubdomainEnabled = v },
		func(s Settings) bool { return s.SubdomainEnabled }),
	boolDefinition(KeyPathEnabled, true,
		func(s *Settings, v bool) { s.PathEnabled = v },
		func(s Settings) bool { return s.PathEnabled }),
	boolDefinition(KeyCustomDomains, true,
		func(s *Settings, v bool) { s.CustomDomainsEnabled = v },
		func(s Settings) bool { return s.CustomDomainsEnabled }),
	intDefinition(KeyMaxDomains, 10, maxDomainsPerSite,
		func(s *Settings, v int64) { s.MaxDomainsPerSite = int(v) },
		func(s Settings) int64 { return int64(s.MaxDomainsPerSite) }),
	readOnlyString(KeyWildcardStatus, WildcardUnchecked, false,
		func(s *Settings, v string) { s.WildcardStatus = v },
		func(s Settings) string { return s.WildcardStatus }),
	readOnlyString(KeyWildcardChecked, "", true,
		func(s *Settings, v string) { s.WildcardCheckedAt = v },
		func(s Settings) string { return s.WildcardCheckedAt }),
	readOnlyString(KeyWildcardDetail, "", false,
		func(s *Settings, v string) { s.WildcardDetail = v },
		func(s Settings) string { return s.WildcardDetail }),
	readOnlyInt(KeyWildcardFailures, 0,
		func(s *Settings, v int64) { s.WildcardFailures = int(v) },
		func(s Settings) int64 { return int64(s.WildcardFailures) }),
	readOnlyString(KeyDisabledReason, "", true,
		func(s *Settings, v string) { s.SubdomainDisabledReason = v },
		func(s Settings) string { return s.SubdomainDisabledReason }),
	stringDefinition(KeyDefaultAccess, AccessPublic, normaliseDefaultAccess,
		func(s *Settings, v string) { s.DefaultAccessMode = v },
		func(s Settings) string { return s.DefaultAccessMode }),
	enumDefinition(KeyWhoCanInvite, InviteRoot, []string{InviteRoot, InviteWorkspaceAdmins},
		func(s *Settings, v string) { s.WhoCanInvite = v },
		func(s Settings) string { return s.WhoCanInvite }),
	enumDefinition(KeyWhoCanCreate, CreateAny, []string{CreateAny, CreateRoot},
		func(s *Settings, v string) { s.WhoCanCreate = v },
		func(s Settings) string { return s.WhoCanCreate }),
}

func init() {
	definitions = append(definitions, emailDefinitions()...)
	definitions = append(definitions, tlsDefinitions()...)
	definitions = append(definitions, analyticsDefinitions()...)
}

func findDefinition(key string) (definition, bool) {
	for _, def := range definitions {
		if def.key == key {
			return def, true
		}
	}
	return definition{}, false
}

func decodeValue(key string, raw json.RawMessage, target any) error {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return validate.Invalid(fmt.Sprintf("Setting %s must not be null.", key))
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return validate.Invalid(fmt.Sprintf("Setting %s has the wrong type.", key))
	}
	return nil
}

func intDefinition(key string, fallback, max int64, set func(*Settings, int64), get func(Settings) int64) definition {
	return definition{
		key:      key,
		fallback: strconv.FormatInt(fallback, 10),
		decode: func(raw json.RawMessage) (string, error) {
			var value int64
			if err := decodeValue(key, raw, &value); err != nil {
				return "", err
			}
			if value < minimumInt || value > max {
				return "", validate.Invalid(fmt.Sprintf("Setting %s must be between %d and %d.", key, minimumInt, max))
			}
			return strconv.FormatInt(value, 10), nil
		},
		apply: func(s *Settings, stored string) error {
			value, err := strconv.ParseInt(stored, 10, 64)
			if err != nil {
				return fmt.Errorf("setting %s holds %q, which is not a whole number", key, stored)
			}
			set(s, value)
			return nil
		},
		value: func(s Settings) any { return get(s) },
	}
}

func boolDefinition(key string, fallback bool, set func(*Settings, bool), get func(Settings) bool) definition {
	return definition{
		key:      key,
		fallback: strconv.FormatBool(fallback),
		decode: func(raw json.RawMessage) (string, error) {
			var value bool
			if err := decodeValue(key, raw, &value); err != nil {
				return "", err
			}
			return strconv.FormatBool(value), nil
		},
		apply: func(s *Settings, stored string) error {
			value, err := strconv.ParseBool(stored)
			if err != nil {
				return fmt.Errorf("setting %s holds %q, which is not true or false", key, stored)
			}
			set(s, value)
			return nil
		},
		value: func(s Settings) any { return get(s) },
	}
}

func stringDefinition(key, fallback string, normalise func(string) (string, error), set func(*Settings, string), get func(Settings) string) definition {
	return definition{
		key:      key,
		fallback: fallback,
		decode: func(raw json.RawMessage) (string, error) {
			var value string
			if err := decodeValue(key, raw, &value); err != nil {
				return "", err
			}
			clean, err := normalise(value)
			if err != nil {
				return "", validate.Invalid(fmt.Sprintf("Setting %s: %s.", key, err.Error()))
			}
			return clean, nil
		},
		apply: func(s *Settings, stored string) error {
			set(s, stored)
			return nil
		},
		value: func(s Settings) any { return get(s) },
	}
}

func readOnlyString(key, fallback string, nullWhenEmpty bool, set func(*Settings, string), get func(Settings) string) definition {
	return definition{
		key:      key,
		fallback: fallback,
		readOnly: true,
		decode: func(json.RawMessage) (string, error) {
			return "", validate.Invalid(fmt.Sprintf("Setting %s is read-only.", key))
		},
		apply: func(s *Settings, stored string) error {
			set(s, stored)
			return nil
		},
		value: func(s Settings) any {
			value := get(s)
			if nullWhenEmpty && value == "" {
				return nil
			}
			return value
		},
	}
}

func readOnlyInt(key string, fallback int64, set func(*Settings, int64), get func(Settings) int64) definition {
	return definition{
		key:      key,
		fallback: strconv.FormatInt(fallback, 10),
		readOnly: true,
		decode: func(json.RawMessage) (string, error) {
			return "", validate.Invalid(fmt.Sprintf("Setting %s is read-only.", key))
		},
		apply: func(s *Settings, stored string) error {
			value, err := strconv.ParseInt(stored, 10, 64)
			if err != nil {
				return fmt.Errorf("setting %s holds %q, which is not a whole number", key, stored)
			}
			set(s, value)
			return nil
		},
		value: func(s Settings) any { return get(s) },
	}
}

func normaliseBaseDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	if domain == "" {
		return "", nil
	}
	if len(domain) > maxBaseDomainLength {
		return "", fmt.Errorf("the domain is longer than %d characters", maxBaseDomainLength)
	}
	for _, label := range strings.Split(domain, ".") {
		if !hostnameLabel.MatchString(label) {
			return "", errors.New("use a bare host name such as example.com, without a scheme, port, path or wildcard")
		}
	}
	return domain, nil
}

func normaliseDefaultAccess(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode != AccessPublic && mode != AccessRestricted {
		return "", errors.New("use public or restricted")
	}
	return mode, nil
}
