package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestInviteLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")

		created := root.invite(createInviteRequest{Email: " Kim@Example.com ", Workspace: "acme", Role: "write"})
		token := inviteToken(t, created.Link)
		if created.Invite.Email != "kim@example.com" || created.Invite.Status != "pending" || created.Invite.InvitedBy != "alex@example.com" {
			t.Errorf("invite = %+v", created.Invite)
		}
		if created.Invite.Workspace == nil || created.Invite.Workspace.Slug != "acme" || created.Invite.Role == nil || *created.Invite.Role != "write" {
			t.Errorf("workspace or role missing: %+v", created.Invite)
		}
		if !created.EmailAvailable || created.EmailSent || created.EmailError != nil {
			t.Errorf("email fields = %+v", created)
		}
		if len(h.mailer.Messages()) != 0 {
			t.Error("an email was sent without sendEmail")
		}

		preview := decodeBody[invitePreviewResponse](t, h.do("GET", "/api/v1/invites/"+token, nil))
		if preview.Email != "kim@example.com" || preview.WorkspaceName == nil || *preview.WorkspaceName != "Acme Ltd" {
			t.Errorf("preview = %+v", preview)
		}

		rec := h.acceptInvite(token, testPassword)
		expectStatus(t, rec, http.StatusCreated)
		cookie := sessionCookie(rec)
		if cookie == nil {
			t.Fatal("accepting an invite set no session cookie")
		}
		user := decodeBody[userEnvelope](t, rec).User
		if user.Email != "kim@example.com" || user.IsRoot || user.DisplayName != "Kim Example" {
			t.Errorf("user = %+v", user)
		}
		me := decodeBody[meResponse](t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)))
		if me.User.ID != user.ID {
			t.Errorf("session belongs to %q, want %q", me.User.ID, user.ID)
		}
		list := decodeBody[workspaceListResponse](t, h.do("GET", "/api/v1/workspaces", nil, withCookie(cookie)))
		roles := map[string]string{}
		for _, ws := range list.Workspaces {
			roles[string(ws.Kind)] = string(ws.Role)
		}
		if len(list.Workspaces) != 2 || roles["personal"] != "admin" || roles["shared"] != "write" {
			t.Errorf("workspaces = %+v", list.Workspaces)
		}
		h.sessionFor("kim@example.com")

		listed := decodeBody[inviteListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/invites", nil))
		if len(listed.Invites) != 1 || listed.Invites[0].Status != "accepted" || listed.Invites[0].AcceptedAt == nil {
			t.Errorf("listed = %+v", listed.Invites)
		}
		expectError(t, h.acceptInvite(token, testPassword), http.StatusNotFound, codeNotFound)
	})
}

func TestInviteLinkIsReturnedWhateverEmailDoes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{noMailer: true})
		root := h.rootActor()
		created := root.invite(createInviteRequest{Email: "kim@example.com", SendEmail: true})
		inviteToken(t, created.Link)
		if created.EmailAvailable || created.EmailSent || created.EmailError != nil {
			t.Errorf("email fields = %+v", created)
		}
	})
}

func TestInviteEmailIsSentOnRequest(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{logToBuf: true})
		root := h.rootActor()
		created := root.invite(createInviteRequest{Email: "kim@example.com", SendEmail: true})
		if !created.EmailAvailable || !created.EmailSent {
			t.Fatalf("created = %+v", created)
		}
		sent := h.mailer.Messages()
		if len(sent) != 1 || sent[0].To != "kim@example.com" || !strings.Contains(sent[0].Text, created.Link) {
			t.Fatalf("messages = %+v", sent)
		}

		h.mailer.FailWith(errString("connection refused by 10.0.0.1"))
		failed := root.invite(createInviteRequest{Email: "lee@example.com", SendEmail: true})
		if failed.EmailSent || failed.EmailError == nil || failed.Link == "" {
			t.Fatalf("failed = %+v", failed)
		}
		if strings.Contains(*failed.EmailError, "10.0.0.1") {
			t.Error("a raw error leaked into the response")
		}
		for _, secret := range []string{inviteToken(t, created.Link), inviteToken(t, failed.Link)} {
			if strings.Contains(h.logs.String(), secret) {
				t.Error("an invite token reached the logs")
			}
		}
	})
}

type errString string

func (e errString) Error() string { return string(e) }

func TestInviteForAnExistingAccountIsAConflict(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		h.bearerActor("sam@example.com")
		body := root.call("POST", "/api/v1/invites", createInviteRequest{Email: "SAM@example.com"})
		got := expectError(t, body, http.StatusConflict, codeEmailRegistered)
		if !strings.Contains(got.Message, "workspace") {
			t.Errorf("message = %q, want advice about adding the user to a workspace", got.Message)
		}
	})
}

func TestInviteValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		for name, body := range map[string]any{
			"bad email":           createInviteRequest{Email: "nope"},
			"empty email":         createInviteRequest{},
			"bad role":            createInviteRequest{Email: "kim@example.com", Workspace: "acme", Role: "owner"},
			"role without target": createInviteRequest{Email: "kim@example.com", Role: "write"},
		} {
			root.expectError(http.StatusBadRequest, codeValidationFailed, "POST", "/api/v1/invites", body)
			_ = name
		}
		root.expectError(http.StatusBadRequest, codeInvalidJSON, "POST", "/api/v1/invites", `{"email":"kim@example.com","extra":1}`)
		root.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/invites", createInviteRequest{Email: "kim@example.com", Workspace: "missing"})
		personal := decodeBody[workspaceListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/workspaces", nil)).Workspaces[0]
		root.expectError(http.StatusConflict, codePersonalWorkspace, "POST", "/api/v1/invites", createInviteRequest{Email: "kim@example.com", Workspace: personal.Slug})
	})
}

func TestInvitePermissionMatrix(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		admin := h.bearerActor("admin@example.com")
		writer := h.bearerActor("writer@example.com")
		outsider := h.bearerActor("outsider@example.com")
		admin.createWorkspace("Acme Ltd", "acme")
		root.createWorkspace("Beta Ltd", "beta")
		admin.addMember("acme", addMemberRequest{Email: "writer@example.com", Role: "write"})
		invite := func(a actor, email, workspace string) *httptest.ResponseRecorder {
			return a.call("POST", "/api/v1/invites", createInviteRequest{Email: email, Workspace: workspace})
		}

		expectError(t, h.do("POST", "/api/v1/invites", createInviteRequest{Email: "x@example.com"}), http.StatusUnauthorized, codeUnauthenticated)
		expectError(t, h.do("GET", "/api/v1/invites", nil), http.StatusUnauthorized, codeUnauthenticated)
		for i, a := range []actor{admin, writer, outsider} {
			expectError(t, invite(a, "root-only-"+string(rune('a'+i))+"@example.com", "acme"), http.StatusForbidden, codeForbidden)
		}
		expectStatus(t, invite(root, "one@example.com", ""), http.StatusCreated)
		expectStatus(t, invite(root, "two@example.com", "beta"), http.StatusCreated)

		root.setSettings(`{"settings":{"invites.whoCanInvite":"workspaceAdmins"}}`)
		expectStatus(t, invite(admin, "three@example.com", "acme"), http.StatusCreated)
		expectError(t, invite(admin, "four@example.com", ""), http.StatusForbidden, codeForbidden)
		expectError(t, invite(admin, "five@example.com", "beta"), http.StatusNotFound, codeNotFound)
		expectError(t, invite(writer, "six@example.com", "acme"), http.StatusForbidden, codeForbidden)
		expectError(t, invite(outsider, "seven@example.com", "acme"), http.StatusNotFound, codeNotFound)
		expectStatus(t, invite(root, "eight@example.com", ""), http.StatusCreated)

		rootList := decodeBody[inviteListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/invites", nil))
		adminList := decodeBody[inviteListResponse](t, admin.expect(http.StatusOK, "GET", "/api/v1/invites", nil))
		writerList := decodeBody[inviteListResponse](t, writer.expect(http.StatusOK, "GET", "/api/v1/invites", nil))
		if len(rootList.Invites) != 4 || len(adminList.Invites) != 1 || adminList.Invites[0].Email != "three@example.com" || len(writerList.Invites) != 0 {
			t.Errorf("lists = %d, %+v, %d", len(rootList.Invites), adminList.Invites, len(writerList.Invites))
		}
		mine := adminList.Invites[0].ID
		writer.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/invites/"+mine, nil)
		writer.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/invites/"+mine+"/resend", nil)
		expectError(t, h.do("DELETE", "/api/v1/invites/"+mine, nil), http.StatusUnauthorized, codeUnauthenticated)
		admin.expect(http.StatusNoContent, "DELETE", "/api/v1/invites/"+mine, nil)
	})
}

func TestUnusableInviteLinksAllLookIdentical(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		expired := inviteToken(t, root.invite(createInviteRequest{Email: "old@example.com"}).Link)
		h.clock.Advance(7*24*time.Hour + time.Second)
		revoked := root.invite(createInviteRequest{Email: "gone@example.com"})
		root.expect(http.StatusNoContent, "DELETE", "/api/v1/invites/"+revoked.Invite.ID, nil)
		used := inviteToken(t, root.invite(createInviteRequest{Email: "done@example.com"}).Link)
		expectStatus(t, h.acceptInvite(used, testPassword), http.StatusCreated)

		tokens := map[string]string{
			"expired": expired, "revoked": inviteToken(t, revoked.Link), "used": used,
			"unknown": strings.Repeat("A", 43), "short": "abc",
		}
		var first []byte
		for name, token := range tokens {
			for _, rec := range []*httptest.ResponseRecorder{h.do("GET", "/api/v1/invites/"+token, nil), h.acceptInvite(token, testPassword)} {
				expectError(t, rec, http.StatusNotFound, codeNotFound)
				if first == nil {
					first = rec.Body.Bytes()
				}
				if !bytes.Equal(rec.Body.Bytes(), first) {
					t.Errorf("%s: body %q differs from %q", name, rec.Body.String(), first)
				}
			}
		}
	})
}

func TestAcceptingNeedsAGoodPasswordAndKeepsTheInvite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		token := inviteToken(t, root.invite(createInviteRequest{Email: "kim@example.com"}).Link)
		expectError(t, h.acceptInvite(token, "short"), http.StatusBadRequest, codeValidationFailed)
		expectError(t, h.do("POST", "/api/v1/invites/"+token+"/accept", `{"password":"`+testPassword+`","email":"other@example.com"}`), http.StatusBadRequest, codeInvalidJSON)
		expectError(t, h.do("POST", "/api/v1/invites/"+token+"/accept", acceptInviteRequest{Password: testPassword, DisplayName: "bad\nname"}), http.StatusBadRequest, codeValidationFailed)
		rec := h.acceptInvite(token, testPassword)
		expectStatus(t, rec, http.StatusCreated)
		if got := decodeBody[userEnvelope](t, rec).User.Email; got != "kim@example.com" {
			t.Errorf("the invitee chose the email %q", got)
		}
	})
}

func TestInviteResendAndRevoke(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		first := root.invite(createInviteRequest{Email: "kim@example.com"})
		resent := decodeBody[createdInviteResponse](t, root.expect(http.StatusCreated, "POST", "/api/v1/invites/"+first.Invite.ID+"/resend", resendInviteRequest{SendEmail: true}))
		if resent.Link == first.Link || !resent.EmailSent || resent.Invite.ID == first.Invite.ID {
			t.Fatalf("resent = %+v", resent)
		}
		expectError(t, h.do("GET", "/api/v1/invites/"+inviteToken(t, first.Link), nil), http.StatusNotFound, codeNotFound)
		expectStatus(t, h.do("GET", "/api/v1/invites/"+inviteToken(t, resent.Link), nil), http.StatusOK)
		again := decodeBody[createdInviteResponse](t, root.expect(http.StatusCreated, "POST", "/api/v1/invites/"+resent.Invite.ID+"/resend", nil))
		if again.EmailSent {
			t.Error("an email was sent although none was requested")
		}

		root.expect(http.StatusNoContent, "DELETE", "/api/v1/invites/"+again.Invite.ID, nil)
		root.expect(http.StatusNoContent, "DELETE", "/api/v1/invites/"+again.Invite.ID, nil)
		root.expectError(http.StatusNotFound, codeNotFound, "DELETE", "/api/v1/invites/missing", nil)
		expectError(t, h.do("GET", "/api/v1/invites/"+inviteToken(t, again.Link), nil), http.StatusNotFound, codeNotFound)

		accepted := root.invite(createInviteRequest{Email: "lee@example.com"})
		expectStatus(t, h.acceptInvite(inviteToken(t, accepted.Link), testPassword), http.StatusCreated)
		root.expectError(http.StatusConflict, codeInviteAccepted, "DELETE", "/api/v1/invites/"+accepted.Invite.ID, nil)
		root.expectError(http.StatusConflict, codeInviteAccepted, "POST", "/api/v1/invites/"+accepted.Invite.ID+"/resend", nil)
		statuses := map[string]string{}
		for _, inv := range decodeBody[inviteListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/invites", nil)).Invites {
			statuses[inv.ID] = inv.Status
		}
		if statuses[first.Invite.ID] != "revoked" || statuses[again.Invite.ID] != "revoked" || statuses[accepted.Invite.ID] != "accepted" {
			t.Errorf("statuses = %v", statuses)
		}
	})
}

func TestInviteEndpointsAreRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{limits: limitsWith(func(l *Limits) {
			l.InviteLookup = Rule{Max: 2, Window: time.Hour}
			l.InviteAccept = Rule{Max: 2, Window: time.Hour}
			l.InviteSend = Rule{Max: 2, Window: time.Hour}
		})})
		root := h.rootActor()
		root.invite(createInviteRequest{Email: "one@example.com"})
		root.invite(createInviteRequest{Email: "two@example.com"})
		root.expectError(http.StatusTooManyRequests, codeRateLimited, "POST", "/api/v1/invites", createInviteRequest{Email: "three@example.com"})

		unknown := strings.Repeat("A", 43)
		for range 2 {
			expectError(t, h.do("GET", "/api/v1/invites/"+unknown, nil), http.StatusNotFound, codeNotFound)
			expectError(t, h.acceptInvite(unknown, testPassword), http.StatusNotFound, codeNotFound)
		}
		expectError(t, h.do("GET", "/api/v1/invites/"+unknown, nil), http.StatusTooManyRequests, codeRateLimited)
		expectError(t, h.acceptInvite(unknown, testPassword), http.StatusTooManyRequests, codeRateLimited)
		expectError(t, h.do("GET", "/api/v1/invites/"+unknown, nil, fromAddr("203.0.113.9:4000")), http.StatusNotFound, codeNotFound)
	})
}

func TestInviteCreationFollowsCSRFRulesForSessions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		body := createInviteRequest{Email: "kim@example.com"}
		expectError(t, h.do("POST", "/api/v1/invites", body, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("POST", "/api/v1/invites", body, withCookie(cookie), browser()), http.StatusCreated)
	})
}
