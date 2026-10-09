package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	IsRoot      bool
	DisabledAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u User) Disabled() bool { return u.DisabledAt != nil }

const userColumns = "id, email, display_name, is_root, disabled_at, created_at, updated_at"

func scanUserFrom(row interface{ Scan(dest ...any) error }, extra ...any) (User, error) {
	var u User
	var disabled sql.NullTime
	dest := append([]any{&u.ID, &u.Email, &u.DisplayName, &u.IsRoot, &disabled, &u.CreatedAt, &u.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return User{}, err
	}
	if disabled.Valid {
		u.DisabledAt = &disabled.Time
	}
	return u, nil
}

func scanUser(row *sql.Row) (User, error) {
	return scanUserFrom(row)
}

func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM users LIMIT 1").Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check for users: %w", err)
	}
	return false, nil
}

func (s *Service) CreateUser(ctx context.Context, email, password string) (User, error) {
	prepared, err := s.PrepareCredentials(email, password)
	if err != nil {
		return User{}, err
	}
	var created User
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		user, insertErr := s.insertUser(ctx, q, prepared, "", false)
		if insertErr != nil {
			return insertErr
		}
		created = user
		return nil
	})
	if store.IsUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, err
	}
	return created, nil
}

func (s *Service) Setup(ctx context.Context, email, password string) (User, error) {
	prepared, err := s.PrepareCredentials(email, password)
	if err != nil {
		return User{}, err
	}
	var created User
	err = store.WithTx(ctx, s.db, func(q store.Querier) error {
		var one int
		scanErr := q.QueryRowContext(ctx, "SELECT 1 FROM users LIMIT 1").Scan(&one)
		if scanErr == nil {
			return ErrSetupComplete
		}
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return fmt.Errorf("check for users: %w", scanErr)
		}
		user, insertErr := s.insertUser(ctx, q, prepared, "", true)
		if insertErr != nil {
			return insertErr
		}
		created = user
		return nil
	})
	if store.IsUniqueViolation(err) {
		return User{}, ErrSetupComplete
	}
	if err != nil {
		return User{}, err
	}
	return created, nil
}

func (s *Service) UserByID(ctx context.Context, id string) (User, error) {
	user, err := scanUser(s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	return user, nil
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return User{}, s.rejectUnknownUser(password)
	}
	var hash string
	row := s.db.QueryRowContext(ctx, "SELECT "+userColumns+", password_hash FROM users WHERE email = $1", normalized)
	user, err := scanUserFrom(row, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, s.rejectUnknownUser(password)
	}
	if err != nil {
		return User{}, fmt.Errorf("load user for login: %w", err)
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		return User{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return User{}, ErrInvalidCredentials
	}
	if user.Disabled() {
		return User{}, ErrUserDisabled
	}
	if NeedsRehash(hash, s.params) {
		s.upgradeHash(ctx, user.ID, password)
	}
	return user, nil
}

type Credentials struct {
	email string
	hash  string
}

func (s *Service) PrepareCredentials(email, password string) (Credentials, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return Credentials{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Credentials{}, err
	}
	hash, err := HashPassword(password, s.params)
	if err != nil {
		return Credentials{}, fmt.Errorf("hash password: %w", err)
	}
	return Credentials{email: normalized, hash: hash}, nil
}

func (s *Service) InsertMember(ctx context.Context, q store.Querier, c Credentials, displayName string) (User, error) {
	return s.insertUser(ctx, q, c, displayName, false)
}

func (s *Service) insertUser(ctx context.Context, q store.Querier, c Credentials, displayName string, isRoot bool) (User, error) {
	id, err := newID()
	if err != nil {
		return User{}, err
	}
	now := s.clock()
	_, err = q.ExecContext(ctx,
		"INSERT INTO users (id, email, display_name, password_hash, is_root, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		id, c.email, displayName, c.hash, isRoot, now, now)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	if _, err := workspaces.CreatePersonal(ctx, q, id, now); err != nil {
		return User{}, fmt.Errorf("create personal workspace: %w", err)
	}
	return User{ID: id, Email: c.email, DisplayName: displayName, IsRoot: isRoot, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Service) rejectUnknownUser(password string) error {
	if _, err := VerifyPassword(password, s.dummyHash); err != nil {
		return fmt.Errorf("verify placeholder hash: %w", err)
	}
	return ErrInvalidCredentials
}

func (s *Service) upgradeHash(ctx context.Context, userID, password string) {
	hash, err := HashPassword(password, s.params)
	if err != nil {
		s.logger.Warn("rehash password", "error", err)
		return
	}
	_, err = s.db.ExecContext(ctx, "UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3", hash, s.clock(), userID)
	if err != nil {
		s.logger.Warn("store rehashed password", "error", err)
	}
}
