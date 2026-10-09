package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSetupComplete      = errors.New("setup is already complete")
	ErrEmailTaken         = errors.New("email is already registered")
	ErrUserNotFound       = errors.New("user not found")
	ErrSessionInvalid     = errors.New("session is missing, unknown or expired")
	ErrAPIKeyInvalid      = errors.New("api key is unknown or revoked")
	ErrAPIKeyNotFound     = errors.New("api key not found")
	ErrAPIKeyLimit        = errors.New("active api key limit reached")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }
