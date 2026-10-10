package domains

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"strings"
)

const (
	maxRecordsRead  = 32
	maxRecordLength = 512
)

func (s *Service) check(ctx context.Context, d Domain) string {
	lookupCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	records, err := s.resolver.LookupTXT(lookupCtx, d.ChallengeName())
	if err != nil {
		return classifyFailure(err)
	}
	if len(records) == 0 {
		return ResultNotFound
	}
	if anyRecordMatches(records, d.Token) {
		return ResultVerified
	}
	return ResultMismatch
}

func classifyFailure(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound && !dnsErr.IsTemporary && !dnsErr.IsTimeout {
		return ResultNotFound
	}
	return ResultLookupFailed
}

func anyRecordMatches(records []string, token string) bool {
	if len(records) > maxRecordsRead {
		records = records[:maxRecordsRead]
	}
	matched := false
	for _, record := range records {
		if len(record) > maxRecordLength {
			continue
		}
		value := strings.Trim(strings.TrimSpace(record), `"`)
		if subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1 {
			matched = true
		}
	}
	return matched
}

func countsAsMissing(result string) bool {
	return result == ResultNotFound || result == ResultMismatch
}
