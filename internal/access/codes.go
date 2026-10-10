package access

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/otp"
)

const (
	codePurpose = "viewer-code"
	sendTimeout = 30 * time.Second
)

type CodeRequest struct {
	SiteID    string
	SiteLabel string
	Email     string
}

func (s *Service) RequestCode(ctx context.Context, in CodeRequest) error {
	email, err := auth.NormalizeEmail(in.Email)
	if err != nil {
		return nil
	}
	allowed, err := s.emailAllowed(ctx, in.SiteID, email)
	if err != nil || !allowed || !s.mailer.Configured() {
		return err
	}
	code, err := otp.New(CodeDigits)
	if err != nil {
		return err
	}
	hash, err := s.codeHash(ctx, in.SiteID, email, code)
	if err != nil {
		return err
	}
	now := s.clock()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO viewer_codes (site_id, email, code_hash, attempts, created_at, expires_at) VALUES ($1, $2, $3, 0, $4, $5)
		ON CONFLICT (site_id, email) DO UPDATE SET code_hash = excluded.code_hash, attempts = 0,
		created_at = excluded.created_at, expires_at = excluded.expires_at`,
		in.SiteID, email, hash, now, now.Add(CodeLifetime))
	if err != nil {
		return wrap("store sign-in code", err)
	}
	s.spawn(func() { s.deliver(in, email, code, hash) })
	return nil
}

func (s *Service) deliver(in CodeRequest, email, code, hash string) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	msg := mail.Message{
		To:      email,
		Subject: "Your sign-in code for " + in.SiteLabel,
		Text: fmt.Sprintf("Your sign-in code for %s is %s.\n\nIt works once and expires in %d minutes. If you did not ask for it, ignore this message.\n",
			in.SiteLabel, code, int(CodeLifetime.Minutes())),
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.logger.Error("send sign-in code", "site_id", in.SiteID, "error", err)
		s.discardCode(ctx, in.SiteID, email, hash)
	}
}

func (s *Service) discardCode(ctx context.Context, siteID, email, hash string) {
	_, err := s.db.ExecContext(ctx, "DELETE FROM viewer_codes WHERE site_id = $1 AND email = $2 AND code_hash = $3", siteID, email, hash)
	if err != nil {
		s.logger.Error("discard unsent sign-in code", "site_id", siteID, "error", err)
	}
}

func (s *Service) VerifyCode(ctx context.Context, siteID, email, code string) (State, bool, error) {
	normalised, err := auth.NormalizeEmail(email)
	state, stateErr := s.State(ctx, siteID)
	if stateErr != nil {
		return State{}, false, stateErr
	}
	if err != nil || state.Mode != ModeRestricted || !otp.ValidShape(code, CodeDigits) {
		return state, false, nil
	}
	matched, err := s.consumeAttempt(ctx, siteID, normalised, code)
	if err != nil || !matched {
		return state, false, err
	}
	allowed, err := s.emailAllowed(ctx, siteID, normalised)
	return state, allowed, err
}

func (s *Service) consumeAttempt(ctx context.Context, siteID, email, code string) (bool, error) {
	held, err := s.reserveAttempt(ctx, siteID, email)
	if err != nil || !held {
		return false, err
	}
	var stored string
	var attempts int
	err = s.db.QueryRowContext(ctx, "SELECT code_hash, attempts FROM viewer_codes WHERE site_id = $1 AND email = $2", siteID, email).
		Scan(&stored, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, wrap("load sign-in code", err)
	}
	given, err := s.codeHash(ctx, siteID, email, code)
	if err != nil {
		return false, err
	}
	if !hmac.Equal([]byte(given), []byte(stored)) {
		return false, s.dropIfSpent(ctx, siteID, email, attempts)
	}
	return s.spendCode(ctx, siteID, email, stored)
}

func (s *Service) reserveAttempt(ctx context.Context, siteID, email string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"UPDATE viewer_codes SET attempts = attempts + 1 WHERE site_id = $1 AND email = $2 AND attempts < $3 AND expires_at > $4",
		siteID, email, CodeAttempts, s.clock())
	if err != nil {
		return false, wrap("count sign-in attempt", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, wrap("count sign-in attempt", err)
	}
	return rows == 1, nil
}

func (s *Service) dropIfSpent(ctx context.Context, siteID, email string, attempts int) error {
	if attempts < CodeAttempts {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM viewer_codes WHERE site_id = $1 AND email = $2", siteID, email)
	if err != nil {
		return wrap("drop spent sign-in code", err)
	}
	return nil
}

func (s *Service) spendCode(ctx context.Context, siteID, email, hash string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM viewer_codes WHERE site_id = $1 AND email = $2 AND code_hash = $3", siteID, email, hash)
	if err != nil {
		return false, wrap("spend sign-in code", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, wrap("spend sign-in code", err)
	}
	return rows == 1, nil
}

func (s *Service) codeHash(ctx context.Context, siteID, email, code string) (string, error) {
	mac, err := s.sign(ctx, codePurpose, siteID+"|"+email+"|"+code)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac), nil
}
