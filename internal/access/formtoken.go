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

func (s *Service) FormToken(ctx context.Context, siteID string, epoch int64, client string) (string, error) {
	return s.formTokenFor(ctx, siteID, epoch, client, s.formBucket(0))
}

func (s *Service) VerifyFormToken(ctx context.Context, siteID string, epoch int64, client, token string) (bool, error) {
	given, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false, nil
	}
	for _, back := range []int64{0, 1} {
		want, err := s.formMAC(ctx, siteID, epoch, client, s.formBucket(back))
		if err != nil {
			return false, err
		}
		if hmac.Equal(given, want) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) formTokenFor(ctx context.Context, siteID string, epoch int64, client string, bucket int64) (string, error) {
	mac, err := s.formMAC(ctx, siteID, epoch, client, bucket)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac), nil
}

func (s *Service) formMAC(ctx context.Context, siteID string, epoch int64, client string, bucket int64) ([]byte, error) {
	message := siteID + "|" + strconv.FormatInt(epoch, 10) + "|" + client + "|" + strconv.FormatInt(bucket, 10)
	return s.sign(ctx, formPurpose, message)
}

func (s *Service) formBucket(back int64) int64 {
	return s.clock().Unix()/int64(formWindow.Seconds()) - back
}
