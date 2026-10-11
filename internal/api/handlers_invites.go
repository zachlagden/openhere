package api

import (
	"net/http"
	"time"

	"github.com/zachlagden/openhere/internal/invites"
	"github.com/zachlagden/openhere/internal/proxy"
)

type inviteWorkspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type inviteResponse struct {
	ID         string           `json:"id"`
	Email      string           `json:"email"`
	Status     string           `json:"status"`
	Workspace  *inviteWorkspace `json:"workspace"`
	Role       *string          `json:"role"`
	InvitedBy  string           `json:"invitedBy"`
	CreatedAt  time.Time        `json:"createdAt"`
	ExpiresAt  time.Time        `json:"expiresAt"`
	AcceptedAt *time.Time       `json:"acceptedAt"`
	RevokedAt  *time.Time       `json:"revokedAt"`
}

type inviteListResponse struct {
	Invites []inviteResponse `json:"invites"`
}

type createdInviteResponse struct {
	Invite         inviteResponse `json:"invite"`
	Link           string         `json:"link"`
	EmailAvailable bool           `json:"emailAvailable"`
	EmailSent      bool           `json:"emailSent"`
	EmailError     *string        `json:"emailError"`
}

type invitePreviewResponse struct {
	Email         string  `json:"email"`
	WorkspaceName *string `json:"workspaceName"`
}

type createInviteRequest struct {
	Email     string `json:"email"`
	Workspace string `json:"workspace"`
	Role      string `json:"role"`
	SendEmail bool   `json:"sendEmail"`
}

type resendInviteRequest struct {
	SendEmail bool `json:"sendEmail"`
}

type acceptInviteRequest struct {
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (s *Server) registerInviteRoutes(mux *routeMux) {
	mux.Handle("GET "+basePath+"/invites", s.protected(s.handleListInvites))
	mux.Handle("POST "+basePath+"/invites", s.protected(s.handleCreateInvite))
	mux.Handle("DELETE "+basePath+"/invites/{id}", s.protected(s.handleRevokeInvite))
	mux.Handle("POST "+basePath+"/invites/{id}/resend", s.protected(s.handleResendInvite))
	mux.Handle("GET "+basePath+"/invites/{token}", s.identify(http.HandlerFunc(s.handleInvitePreview)))
	mux.Handle("POST "+basePath+"/invites/{token}/accept", s.identify(http.HandlerFunc(s.handleAcceptInvite)))
}

func toInviteResponse(inv invites.Invite, now time.Time) inviteResponse {
	response := inviteResponse{
		ID: inv.ID, Email: inv.Email, Status: inv.Status(now), InvitedBy: inv.InvitedByEmail,
		CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt, RevokedAt: inv.RevokedAt,
	}
	if inv.WorkspaceID != "" {
		role := string(inv.Role)
		response.Workspace = &inviteWorkspace{ID: inv.WorkspaceID, Slug: inv.WorkspaceSlug, Name: inv.WorkspaceName}
		response.Role = &role
	}
	return response
}

func (s *Server) toCreatedInvite(created invites.Created) createdInviteResponse {
	response := createdInviteResponse{
		Invite: toInviteResponse(created.Invite, s.now()), Link: created.Link,
		EmailAvailable: created.EmailAvailable, EmailSent: created.EmailSent,
	}
	if created.EmailError != "" {
		reason := created.EmailError
		response.EmailError = &reason
	}
	return response
}

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	list, err := s.invites.List(r.Context(), p.user)
	if err != nil {
		s.writeInternal(w, r, err)
		return
	}
	now := s.now()
	response := inviteListResponse{Invites: make([]inviteResponse, 0, len(list))}
	for _, inv := range list {
		response.Invites = append(response.Invites, toInviteResponse(inv, now))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req createInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.allow(w, s.inviteSend, p.user.ID) {
		return
	}
	created, err := s.invites.Create(r.Context(), p.user, invites.CreateInput{
		Email: req.Email, Workspace: req.Workspace, Role: req.Role, SendEmail: req.SendEmail,
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.toCreatedInvite(created))
}

func (s *Server) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.invites.Revoke(r.Context(), p.user, r.PathValue("id")); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResendInvite(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req resendInviteRequest
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	if !s.allow(w, s.inviteSend, p.user.ID) {
		return
	}
	created, err := s.invites.Resend(r.Context(), p.user, r.PathValue("id"), req.SendEmail)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.toCreatedInvite(created))
}

func (s *Server) handleInvitePreview(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.inviteLookup, proxy.LimiterKey(s.proxies.ClientIP(r))) {
		return
	}
	preview, err := s.invites.Preview(r.Context(), r.PathValue("token"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	response := invitePreviewResponse{Email: preview.Email}
	if preview.WorkspaceName != "" {
		response.WorkspaceName = &preview.WorkspaceName
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, s.inviteAccept, proxy.LimiterKey(s.proxies.ClientIP(r))) {
		return
	}
	var req acceptInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.invites.Accept(r.Context(), r.PathValue("token"), req.Password, req.DisplayName)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	s.startSession(w, r, user, http.StatusCreated)
}
