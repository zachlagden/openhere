package access

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zachlagden/selfhost/internal/auth"
)

const timingEqualiserPassword = "timing-equaliser-not-a-real-password"

func (s *Service) CheckPassword(ctx context.Context, siteID, password string) (State, bool, error) {
	var mode string
	var state State
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT access_mode, access_epoch, access_password_hash FROM sites WHERE id = $1", siteID).
		Scan(&mode, &state.Epoch, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return State{}, false, ErrSiteNotFound
	}
	if err != nil {
		return State{}, false, wrap("load site password", err)
	}
	state.Mode = Mode(mode)
	if state.Mode != ModePassword || !hash.Valid {
		return state, false, s.spendVerifyTime(password)
	}
	ok, err := auth.VerifyPassword(password, hash.String)
	if err != nil {
		return state, false, wrap("verify site password", err)
	}
	return state, ok, nil
}

func (s *Service) spendVerifyTime(password string) error {
	s.dummyOnce.Do(func() {
		s.dummyHash, s.dummyErr = auth.HashPassword(timingEqualiserPassword, s.params)
	})
	if s.dummyErr != nil {
		return wrap("equalise verify time", s.dummyErr)
	}
	if _, err := auth.VerifyPassword(password, s.dummyHash); err != nil {
		return wrap("equalise verify time", err)
	}
	return nil
}
