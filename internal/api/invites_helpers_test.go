package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func generousLimits() Limits {
	generous := Rule{Max: 1000, Window: time.Hour}
	return Limits{
		LoginPerIP: generous, LoginPerEmail: generous, SetupPerIP: generous, SetupPerEmail: generous,
		EmailTest: generous, InviteSend: generous, InviteLookup: generous, InviteAccept: generous,
		ResetRequestIP: generous, ResetRequestEmail: generous, ResetConfirm: generous,
	}
}

func limitsWith(change func(*Limits)) *Limits {
	limits := generousLimits()
	change(&limits)
	return &limits
}

func (a actor) invite(body createInviteRequest) createdInviteResponse {
	a.h.t.Helper()
	return decodeBody[createdInviteResponse](a.h.t, a.expect(http.StatusCreated, "POST", "/api/v1/invites", body))
}

func inviteToken(t *testing.T, link string) string {
	t.Helper()
	token, found := strings.CutPrefix(link, "http://"+testHost+"/invite/")
	if !found || token == "" {
		t.Fatalf("link %q is not an invite link", link)
	}
	return token
}

func (h *harness) acceptInvite(token, password string) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/v1/invites/"+token+"/accept", acceptInviteRequest{Password: password, DisplayName: "Kim Example"})
}
