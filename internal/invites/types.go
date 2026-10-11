package invites

import (
	"errors"
	"time"

	"github.com/zachlagden/openhere/internal/workspaces"
)

const (
	Lifetime = 7 * 24 * time.Hour

	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRevoked  = "revoked"
	StatusExpired  = "expired"

	maxListed = 500
)

var (
	ErrInviteInvalid   = errors.New("the invite is unknown, expired, revoked or already used")
	ErrInviteNotFound  = errors.New("invite not found")
	ErrNotPermitted    = errors.New("your account may not send invitations here")
	ErrAccepted        = errors.New("the invite has already been accepted")
	ErrEmailRegistered = errors.New("that email address already has an account")
)

type Invite struct {
	ID             string
	Email          string
	InvitedByID    string
	InvitedByEmail string
	WorkspaceID    string
	WorkspaceSlug  string
	WorkspaceName  string
	Role           workspaces.Role
	CreatedAt      time.Time
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	RevokedAt      *time.Time
}

func (i Invite) Status(now time.Time) string {
	switch {
	case i.AcceptedAt != nil:
		return StatusAccepted
	case i.RevokedAt != nil:
		return StatusRevoked
	case !i.ExpiresAt.After(now):
		return StatusExpired
	default:
		return StatusPending
	}
}

type CreateInput struct {
	Email     string
	Workspace string
	Role      string
	SendEmail bool
}

type Created struct {
	Invite         Invite
	Link           string
	EmailAvailable bool
	EmailSent      bool
	EmailError     string
}

type Preview struct {
	Email         string
	WorkspaceName string
}
