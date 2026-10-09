package api

import (
	"time"

	"github.com/zachlagden/selfhost/internal/auth"
)

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	IsRoot    bool      `json:"isRoot"`
	CreatedAt time.Time `json:"createdAt"`
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, IsRoot: u.IsRoot, CreatedAt: u.CreatedAt}
}

type apiKeyResponse struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

func toAPIKeyResponse(k auth.APIKey) apiKeyResponse {
	return apiKeyResponse{ID: k.ID, Label: k.Label, Prefix: k.Prefix, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt, RevokedAt: k.RevokedAt}
}
