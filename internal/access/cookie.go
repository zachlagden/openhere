package access

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

const (
	cookiePrefix    = "v1"
	cookiePurpose   = "viewer-cookie"
	maxCookieLength = 512
	claimFields     = 5
)

type Claims struct {
	SiteID  string
	Epoch   int64
	Expires time.Time
	Method  Method
	Subject string
}

func (s *Service) IssueCookie(_ context.Context, siteID string, epoch int64, method Method, subject string) (string, time.Time, error) {
	expires := s.clock().Add(CookieLifetime)
	payload := strings.Join([]string{siteID, strconv.FormatInt(epoch, 10), strconv.FormatInt(expires.Unix(), 10), string(method), subject}, "|")
	mac := s.sign(cookiePurpose, payload)
	encoding := base64.RawURLEncoding
	value := cookiePrefix + "." + encoding.EncodeToString([]byte(payload)) + "." + encoding.EncodeToString(mac)
	return value, expires, nil
}

func (s *Service) ParseCookie(_ context.Context, value string) (Claims, bool, error) {
	parts := strings.Split(value, ".")
	if len(value) > maxCookieLength || len(parts) != 3 || parts[0] != cookiePrefix {
		return Claims{}, false, nil
	}
	encoding := base64.RawURLEncoding
	payload, payloadErr := encoding.DecodeString(parts[1])
	given, macErr := encoding.DecodeString(parts[2])
	if payloadErr != nil || macErr != nil {
		return Claims{}, false, nil
	}
	want := s.sign(cookiePurpose, string(payload))
	if !hmac.Equal(given, want) {
		return Claims{}, false, nil
	}
	claims, ok := decodeClaims(string(payload))
	return claims, ok, nil
}

func decodeClaims(payload string) (Claims, bool) {
	fields := strings.Split(payload, "|")
	if len(fields) != claimFields {
		return Claims{}, false
	}
	epoch, epochErr := strconv.ParseInt(fields[1], 10, 64)
	expiry, expiryErr := strconv.ParseInt(fields[2], 10, 64)
	method := Method(fields[3])
	if epochErr != nil || expiryErr != nil || !method.valid() {
		return Claims{}, false
	}
	return Claims{SiteID: fields[0], Epoch: epoch, Expires: time.Unix(expiry, 0).UTC(), Method: method, Subject: fields[4]}, true
}
