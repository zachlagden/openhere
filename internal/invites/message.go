package invites

import (
	"fmt"
	"net/url"
	"time"

	"github.com/zachlagden/selfhost/internal/mail"
)

const sendTimeout = 30 * time.Second

func reasonOf(err error) string {
	return mail.Reason(err)
}

func (s *Service) host() string {
	parsed, err := url.Parse(s.baseURL)
	if err != nil || parsed.Host == "" {
		return s.baseURL
	}
	return parsed.Host
}

func (s *Service) message(inv Invite, link string) mail.Message {
	place := s.host()
	if inv.WorkspaceName != "" {
		place = fmt.Sprintf("%s and the workspace %s", s.host(), inv.WorkspaceName)
	}
	return mail.Message{
		To:      inv.Email,
		Subject: "You are invited to " + s.host(),
		Text: fmt.Sprintf("%s invited you to join %s.\n\nAccept the invitation and choose a password:\n%s\n\nThe link works once and expires on %s. If you did not expect this message, ignore it.\n",
			inv.InvitedByEmail, place, link, inv.ExpiresAt.Format("2 January 2006")),
	}
}
