package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/validate"
)

const (
	KeyAnalyticsDefault   = "analytics.enabledByDefault"
	KeyAnalyticsRetention = "analytics.retentionDays"
	KeyAnalyticsRawDays   = "analytics.rawDays"
	KeyMaxMindAccountID   = "analytics.maxmindAccountId"
	KeyMaxMindLicenseKey  = "analytics.maxmindLicenseKey"
	KeyGeoStatus          = "analytics.geoStatus"
	KeyGeoUpdatedAt       = "analytics.geoUpdatedAt"
	KeyGeoCheckedAt       = "analytics.geoCheckedAt"
	KeyGeoDetail          = "analytics.geoDetail"

	GeoUnconfigured = "unconfigured"
	GeoOK           = "ok"
	GeoFailed       = "failed"

	DefaultRetentionDays = 395
	DefaultRawDays       = 31
	MinRetentionDays     = 30
	MaxRetentionDays     = 3650
	MinRawDays           = 14
	MaxRawDays           = 365
	maxAccountIDLength   = 20
)

type Analytics struct {
	EnabledByDefault  bool
	RetentionDays     int
	RawDays           int
	MaxMindAccountID  string
	MaxMindLicenseKey string
	GeoStatus         string
	GeoUpdatedAt      string
	GeoCheckedAt      string
	GeoDetail         string
}

func analyticsDefinitions() []definition {
	return []definition{
		boolDefinition(KeyAnalyticsDefault, true,
			func(s *Settings, v bool) { s.Analytics.EnabledByDefault = v },
			func(s Settings) bool { return s.Analytics.EnabledByDefault }),
		boundedIntDefinition(KeyAnalyticsRetention, DefaultRetentionDays, true, MinRetentionDays, MaxRetentionDays,
			func(s *Settings, v int64) { s.Analytics.RetentionDays = int(v) },
			func(s Settings) int64 { return int64(s.Analytics.RetentionDays) }),
		boundedIntDefinition(KeyAnalyticsRawDays, DefaultRawDays, false, MinRawDays, MaxRawDays,
			func(s *Settings, v int64) { s.Analytics.RawDays = int(v) },
			func(s Settings) int64 { return int64(s.Analytics.RawDays) }),
		stringDefinition(KeyMaxMindAccountID, "", normaliseAccountID,
			func(s *Settings, v string) { s.Analytics.MaxMindAccountID = v },
			func(s Settings) string { return s.Analytics.MaxMindAccountID }),
		secretDefinition(KeyMaxMindLicenseKey,
			func(s *Settings, v string) { s.Analytics.MaxMindLicenseKey = v },
			func(s Settings) string { return s.Analytics.MaxMindLicenseKey }),
		readOnlyString(KeyGeoStatus, GeoUnconfigured, false,
			func(s *Settings, v string) { s.Analytics.GeoStatus = v },
			func(s Settings) string { return s.Analytics.GeoStatus }),
		readOnlyString(KeyGeoUpdatedAt, "", true,
			func(s *Settings, v string) { s.Analytics.GeoUpdatedAt = v },
			func(s Settings) string { return s.Analytics.GeoUpdatedAt }),
		readOnlyString(KeyGeoCheckedAt, "", true,
			func(s *Settings, v string) { s.Analytics.GeoCheckedAt = v },
			func(s Settings) string { return s.Analytics.GeoCheckedAt }),
		readOnlyString(KeyGeoDetail, "", false,
			func(s *Settings, v string) { s.Analytics.GeoDetail = v },
			func(s Settings) string { return s.Analytics.GeoDetail }),
	}
}

func boundedIntDefinition(key string, fallback int64, zeroAllowed bool, min, max int64, set func(*Settings, int64), get func(Settings) int64) definition {
	def := intDefinition(key, fallback, max, set, get)
	def.decode = func(raw json.RawMessage) (string, error) {
		var value int64
		if err := decodeValue(key, raw, &value); err != nil {
			return "", err
		}
		if zeroAllowed && value == 0 {
			return "0", nil
		}
		if value < min || value > max {
			zero := ""
			if zeroAllowed {
				zero = "0 or "
			}
			return "", validate.Invalid(fmt.Sprintf("Setting %s must be %sbetween %d and %d.", key, zero, min, max))
		}
		return strconv.FormatInt(value, 10), nil
	}
	return def
}

func normaliseAccountID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if len(id) > maxAccountIDLength {
		return "", fmt.Errorf("use at most %d digits", maxAccountIDLength)
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", errors.New("use the numeric account ID from your MaxMind account")
		}
	}
	return id, nil
}

type GeoState struct {
	Status  string
	Detail  string
	Updated bool
}

func (s *Service) RecordGeo(ctx context.Context, state GeoState) error {
	stamp := s.now().UTC().Truncate(time.Second).Format(time.RFC3339)
	changes := map[string]string{
		KeyGeoStatus:    state.Status,
		KeyGeoDetail:    state.Detail,
		KeyGeoCheckedAt: stamp,
	}
	if state.Updated {
		changes[KeyGeoUpdatedAt] = stamp
	}
	err := s.write(ctx, s.db, changes)
	s.invalidate()
	return err
}
