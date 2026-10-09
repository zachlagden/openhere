package agentcode

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/otp"
)

func (s *Service) Request(ctx context.Context, email string) error {
	if !s.mailer.Configured() {
		return ErrEmailNotConfigured
	}
	user, err := s.auth.UserByEmail(ctx, email)
	if errors.Is(err, auth.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.Disabled() {
		return nil
	}
	code, err := otp.New(CodeDigits)
	if err != nil {
		return err
	}
	hash := s.hash(user.Email, code)
	now := s.clock()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO agent_codes (email, code_hash, attempts, created_at, expires_at) VALUES ($1, $2, 0, $3, $4)
		ON CONFLICT (email) DO UPDATE SET code_hash = excluded.code_hash, attempts = 0,
		created_at = excluded.created_at, expires_at = excluded.expires_at`,
		user.Email, hash, now, now.Add(CodeLifetime))
	if err != nil {
		return fmt.Errorf("store agent code: %w", err)
	}
	s.spawn(func() { s.deliver(user.Email, code, hash) })
	return nil
}

func (s *Service) deliver(address, code, hash string) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	msg := mail.Message{
		To:      address,
		Subject: "Sign-in code for an agent on " + s.host,
		Text: fmt.Sprintf("An agent asked for access to your account on %s. If that was you, give it this code:\n\n%s\n\nThe code works once and expires in %d minutes. It lets the agent create a full-access API key for your account. If you did not expect this, ignore this message.\n",
			s.host, code, int(CodeLifetime/time.Minute)),
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.logger.Warn("send agent sign-in code", "reason", mail.Reason(err))
		s.discard(ctx, address, hash)
	}
}

func (s *Service) discard(ctx context.Context, address, hash string) {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM agent_codes WHERE email = $1 AND code_hash = $2", address, hash); err != nil {
		s.logger.Error("discard unsent agent code", "error", err)
	}
}

func (s *Service) hash(email, code string) string {
	return hex.EncodeToString(s.secrets.Sign(signPurpose, email+"|"+code))
}

func (s *Service) Purge(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM agent_codes WHERE expires_at <= $1", s.clock()); err != nil {
		return fmt.Errorf("purge agent codes: %w", err)
	}
	return nil
}
