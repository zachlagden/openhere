package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zachlagden/selfhost/internal/store"
)

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+userColumns+" FROM users ORDER BY email")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := []User{}
	for rows.Next() {
		user, scanErr := scanUserFrom(rows)
		if scanErr != nil {
			return nil, errors.Join(fmt.Errorf("scan user: %w", scanErr), rows.Close())
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("read users: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close user rows: %w", err)
	}
	return users, nil
}

func (s *Service) UserByEmail(ctx context.Context, email string) (User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	user, err := scanUser(s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE email = $1", normalized))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user by email: %w", err)
	}
	return user, nil
}

func (s *Service) SetDisabled(ctx context.Context, actorID, targetID string, disabled bool) (User, error) {
	var updated User
	err := store.WithTx(ctx, s.db, func(q store.Querier) error {
		target, err := scanUser(q.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", targetID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("load user: %w", err)
		}
		if disabled {
			if err := s.checkDisable(ctx, q, actorID, target); err != nil {
				return err
			}
		}
		updated, err = s.writeDisabled(ctx, q, target, disabled)
		return err
	})
	if err != nil {
		return User{}, err
	}
	return updated, nil
}

func (s *Service) checkDisable(ctx context.Context, q store.Querier, actorID string, target User) error {
	if target.ID == actorID {
		return ErrSelfDisable
	}
	if !target.IsRoot {
		return nil
	}
	var others int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_root AND disabled_at IS NULL AND id <> $1", target.ID).Scan(&others)
	if err != nil {
		return fmt.Errorf("count root users: %w", err)
	}
	if others == 0 {
		return ErrLastRoot
	}
	return nil
}

func (s *Service) writeDisabled(ctx context.Context, q store.Querier, target User, disabled bool) (User, error) {
	now := s.clock()
	var at any
	if disabled {
		at = now
	}
	if _, err := q.ExecContext(ctx, "UPDATE users SET disabled_at = $1, updated_at = $2 WHERE id = $3", at, now, target.ID); err != nil {
		return User{}, fmt.Errorf("update user status: %w", err)
	}
	if disabled {
		if _, err := q.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", target.ID); err != nil {
			return User{}, fmt.Errorf("end sessions: %w", err)
		}
		target.DisabledAt = &now
	} else {
		target.DisabledAt = nil
	}
	target.UpdatedAt = now
	return target, nil
}
