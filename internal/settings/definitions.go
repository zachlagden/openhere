package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zachlagden/selfhost/internal/validate"
)

const (
	KeyKeepVersions     = "retention.keepVersions"
	KeyMaxFilesPerVer   = "limits.maxFilesPerVersion"
	KeyMaxVersionBytes  = "limits.maxVersionBytes"
	KeyMaxFileBytes     = "limits.maxFileBytes"
	KeyBaseDomain       = "hosting.baseDomain"
	KeySubdomainEnabled = "hosting.subdomainEnabled"
	KeyPathEnabled      = "hosting.pathEnabled"

	minimumInt          = 1
	maxBaseDomainLength = 253
	maxKeepVersions     = 10000
	maxFilesLimit       = 1_000_000
	maxBytesLimit       = int64(1) << 50
)

var hostnameLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type Settings struct {
	KeepVersions     int
	MaxFilesPerVer   int
	MaxVersionBytes  int64
	MaxFileBytes     int64
	BaseDomain       string
	SubdomainEnabled bool
	PathEnabled      bool
}

type definition struct {
	key      string
	fallback string
	decode   func(raw json.RawMessage) (string, error)
	apply    func(s *Settings, stored string) error
	value    func(s Settings) any
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
	stringDefinition(KeyBaseDomain, "", normaliseBaseDomain,
		func(s *Settings, v string) { s.BaseDomain = v },
		func(s Settings) string { return s.BaseDomain }),
	boolDefinition(KeySubdomainEnabled, false,
		func(s *Settings, v bool) { s.SubdomainEnabled = v },
		func(s Settings) bool { return s.SubdomainEnabled }),
	boolDefinition(KeyPathEnabled, true,
		func(s *Settings, v bool) { s.PathEnabled = v },
		func(s Settings) bool { return s.PathEnabled }),
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
