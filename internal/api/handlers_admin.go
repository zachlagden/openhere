package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/zachlagden/selfhost/internal/mail"
)

const (
	codeEmailNotConfigured = "email_not_configured"
	codeEmailSendFailed    = "email_send_failed"
	emailTestTimeout       = 30 * time.Second
)

type emailTestResponse struct {
	Sent bool   `json:"sent"`
	To   string `json:"to"`
}

func (s *Server) registerAdminRoutes(mux *http.ServeMux) {
	mux.Handle("POST "+basePath+"/admin/email/test", s.protected(s.handleEmailTest))
	mux.Handle("GET "+basePath+"/admin/users", s.protected(s.handleListUsers))
	mux.Handle("PATCH "+basePath+"/admin/users/{id}", s.protected(s.handleUpdateUser))
	mux.Handle("POST "+basePath+"/admin/users/{id}/password-reset", s.protected(s.handleAdminResetLink))
}

func (s *Server) handleEmailTest(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r.Context())
	if !requireRoot(w, r) {
		return
	}
	if !s.allow(w, s.emailTestLimit, p.user.ID) {
		return
	}
	if !s.mailer.Configured() {
		writeError(w, http.StatusConflict, codeEmailNotConfigured, "Email is not configured. Choose a provider and fill in its required fields first.")
		return
	}
	ctx, cancel := contextWithTimeout(r, emailTestTimeout)
	defer cancel()
	err := s.mailer.Send(ctx, mail.Message{
		To:      p.user.Email,
		Subject: "Test email from " + s.public.Authority(),
		Text:    "This is a test message from " + s.public.String() + ". Email sending works.\n",
	})
	if errors.Is(err, mail.ErrNotConfigured) {
		writeError(w, http.StatusConflict, codeEmailNotConfigured, "Email is not configured. Choose a provider and fill in its required fields first.")
		return
	}
	if err != nil {
		s.logger.Warn("send test email", "reason", mail.Reason(err))
		writeError(w, http.StatusBadGateway, codeEmailSendFailed, "The test email could not be sent: "+mail.Reason(err)+".")
		return
	}
	writeJSON(w, http.StatusOK, emailTestResponse{Sent: true, To: p.user.Email})
}
