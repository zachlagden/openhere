package access

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zachlagden/selfhost/internal/workspaces"
)

type Decision struct {
	State
	Allowed bool
}

func (s *Service) State(ctx context.Context, siteID string) (State, error) {
	var mode string
	var state State
	err := s.db.QueryRowContext(ctx, "SELECT access_mode, access_epoch FROM sites WHERE id = $1", siteID).Scan(&mode, &state.Epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return State{}, ErrSiteNotFound
	}
	if err != nil {
		return State{}, wrap("load site access", err)
	}
	state.Mode = Mode(mode)
	return state, nil
}

func (s *Service) Check(ctx context.Context, siteID string, cookies []string) (Decision, error) {
	state, err := s.State(ctx, siteID)
	if err != nil {
		return Decision{}, err
	}
	if state.Mode == ModePublic {
		return Decision{State: state, Allowed: true}, nil
	}
	for _, value := range cookies {
		claims, ok, err := s.ParseCookie(ctx, value)
		if err != nil {
			return Decision{}, err
		}
		if !ok {
			continue
		}
		valid, err := s.claimsHold(ctx, claims, siteID, state)
		if err != nil {
			return Decision{}, err
		}
		if valid {
			return Decision{State: state, Allowed: true}, nil
		}
	}
	return Decision{State: state}, nil
}

func (s *Service) claimsHold(ctx context.Context, claims Claims, siteID string, state State) (bool, error) {
	if claims.SiteID != siteID || claims.Epoch != state.Epoch || !claims.Method.fits(state.Mode) {
		return false, nil
	}
	if !claims.Expires.After(s.clock()) {
		return false, nil
	}
	if claims.Method == MethodMember {
		return s.UserCanRead(ctx, claims.Subject, siteID)
	}
	return true, nil
}

func (s *Service) UserCanRead(ctx context.Context, userID, siteID string) (bool, error) {
	query := "SELECT 1 FROM sites s WHERE s.id = $1 AND " + workspaces.SiteCoverSQL("s", "$2")
	var one int
	err := s.db.QueryRowContext(ctx, query, siteID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, wrap("check member access", err)
	}
	return true, nil
}
