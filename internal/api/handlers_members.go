package api

import (
	"net/http"

	"github.com/zachlagden/openhere/internal/workspaces"
)

type addMemberRequest struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	ScopeType string `json:"scopeType"`
	ScopeID   string `json:"scopeId"`
}

type updateMemberRequest struct {
	Role string `json:"role"`
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	members, err := s.workspaces.ListMembers(r.Context(), p.user.ID, r.PathValue("workspace"))
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	response := memberListResponse{Members: make([]memberResponse, 0, len(members))}
	for _, m := range members {
		response.Members = append(response.Members, toMemberResponse(m))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req addMemberRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	member, err := s.workspaces.AddMember(r.Context(), p.user.ID, r.PathValue("workspace"), workspaces.AddMemberInput{
		Email: req.Email, Role: req.Role, ScopeType: req.ScopeType, ScopeID: req.ScopeID,
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, memberEnvelope{Member: toMemberResponse(member)})
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	var req updateMemberRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	member, err := s.workspaces.UpdateMember(r.Context(), p.user.ID, r.PathValue("workspace"), r.PathValue("memberId"), req.Role)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, memberEnvelope{Member: toMemberResponse(member)})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if err := s.workspaces.RemoveMember(r.Context(), p.user.ID, r.PathValue("workspace"), r.PathValue("memberId")); err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
