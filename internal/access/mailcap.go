package access

import (
	"context"
	"time"
)

const (
	codeMailKey     = "gate-code-mails"
	capLogInterval  = 10 * time.Minute
	capSettingLabel = "limits.maxGateCodeMailsPerHour"
)

func (s *Service) reserveCodeMail(ctx context.Context) (bool, error) {
	if s.mailCap == nil {
		return true, nil
	}
	limit, err := s.mailCap(ctx)
	if err != nil {
		return false, wrap("read the code mail cap", err)
	}
	if limit <= 0 {
		return true, nil
	}
	s.mailMu.Lock()
	defer s.mailMu.Unlock()
	if s.mailCount.Count(codeMailKey) >= limit {
		s.noteCapReached(limit)
		return false, nil
	}
	s.mailCount.Record(codeMailKey)
	return true, nil
}

func (s *Service) noteCapReached(limit int) {
	now := s.clock()
	if now.Sub(s.capLogged) < capLogInterval {
		return
	}
	s.capLogged = now
	s.logger.Warn("sign-in code emails are capped for this hour", "setting", capSettingLabel, "limit", limit)
}
