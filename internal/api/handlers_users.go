package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachlagden/openhere/internal/auth"
)

type adminUserResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"displayName"`
	IsRoot      bool       `json:"isRoot"`
	Disabled    bool       `json:"disabled"`
	DisabledAt  *time.Time `json:"disabledAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type adminUserEnvelope struct {
	User adminUserResponse `json:"user"`
}

type adminUserListResponse struct {
	Users []adminUserResponse `json:"users"`
}

type updateUserRequest struct {
	Disabled *bool `json:"disabled"`
}

func toAdminUserResponse(u auth.User) adminUserResponse {
	return adminUserResponse{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, IsRoot: u.IsRoot, Disabled: u.Disabled(),
		DisabledAt: u.DisabledAt, CreatedAt: u.CreatedAt,
	}
}

func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	users, err := s.auth.ListUsers(r.Context())
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	response := adminUserListResponse{Users: make([]adminUserResponse, 0, len(users))}
	for _, user := range users {
		response.Users = append(response.Users, toAdminUserResponse(user))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if !requireRoot(w, r) {
		return
	}
	var req updateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Disabled == nil {
		writeError(w, http.StatusBadRequest, codeValidationFailed, "Send disabled as true or false.")
		return
	}
	user, err := s.auth.SetDisabled(r.Context(), p.user.ID, r.PathValue("id"), *req.Disabled)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, adminUserEnvelope{User: toAdminUserResponse(user)})
}
