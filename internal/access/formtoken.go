package access

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"strconv"
	"time"
)

const (
	formPurpose = "gate-form"
	formWindow  = time.Hour
)

func (s *Service) FormToken(_ context.Context, siteID string, epoch int64, client string) (string, error) {
	return s.formTokenFor(siteID, epoch, client, s.formBucket(0)), nil
}

func (s *Service) VerifyFormToken(_ context.Context, siteID string, epoch int64, client, token string) (bool, error) {
	given, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false, nil
	}
	for _, back := range []int64{0, 1} {
		want := s.formMAC(siteID, epoch, client, s.formBucket(back))
		if hmac.Equal(given, want) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) formTokenFor(siteID string, epoch int64, client string, bucket int64) string {
	return base64.RawURLEncoding.EncodeToString(s.formMAC(siteID, epoch, client, bucket))
}

func (s *Service) formMAC(siteID string, epoch int64, client string, bucket int64) []byte {
	message := siteID + "|" + strconv.FormatInt(epoch, 10) + "|" + client + "|" + strconv.FormatInt(bucket, 10)
	return s.sign(formPurpose, message)
}

func (s *Service) formBucket(back int64) int64 {
	return s.clock().Unix()/int64(formWindow.Seconds()) - back
}
