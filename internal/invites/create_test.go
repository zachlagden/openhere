package invites

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/auth"
	"github.com/zachlagden/selfhost/internal/mail"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/validate"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

func TestCreateReturnsTheLinkAndStoresOnlyAHash(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		created, err := f.svc.Create(context.Background(), f.root, CreateInput{Email: " Kim@Example.com "})
		if err != nil {
			t.Fatal(err)
		}
		token := tokenOf(t, created.Link)
		if created.Invite.Email != "kim@example.com" || created.Invite.WorkspaceID != "" {
			t.Errorf("invite = %+v", created.Invite)
		}
		if got := created.Invite.ExpiresAt.Sub(created.Invite.CreatedAt); got != Lifetime {
			t.Errorf("lifetime = %v, want %v", got, Lifetime)
		}
		var stored string
		if err := db.QueryRowContext(context.Background(), "SELECT token_hash FROM invites WHERE id = $1", created.Invite.ID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored == token || stored != auth.HashToken(token) {
			t.Errorf("stored token hash = %q", stored)
		}
		if created.EmailSent || len(f.mailer.Messages()) != 0 {
			t.Error("an email was sent although the request did not ask for one")
		}
		if !created.EmailAvailable {
			t.Error("the recorder is configured, so email must be available")
		}
	})
}

func TestCreateSendsEmailOnlyWhenAskedAndAvailable(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ws := f.workspace(f.root, "Acme Ltd", "acme")
		created, err := f.svc.Create(context.Background(), f.root, CreateInput{Email: "kim@example.com", Workspace: ws.Slug, Role: "write", SendEmail: true})
		if err != nil {
			t.Fatal(err)
		}
		if !created.EmailSent || created.EmailError != "" {
			t.Fatalf("created = %+v", created)
		}
		sent := f.mailer.Messages()
		if len(sent) != 1 || sent[0].To != "kim@example.com" || !strings.Contains(sent[0].Text, created.Link) || !strings.Contains(sent[0].Text, "Acme Ltd") {
			t.Fatalf("messages = %+v", sent)
		}
		if !strings.Contains(sent[0].Text, "alex@example.com") || !strings.Contains(sent[0].Text, "8 March 2026") {
			t.Errorf("message text = %q", sent[0].Text)
		}

		f.mailer.FailWith(errors.New("smtp is down"))
		failed, err := f.svc.Create(context.Background(), f.root, CreateInput{Email: "lee@example.com", SendEmail: true})
		if err != nil {
			t.Fatal(err)
		}
		if failed.EmailSent || failed.EmailError == "" || failed.Link == "" {
			t.Errorf("failed send = %+v", failed)
		}
		if strings.Contains(failed.EmailError, "smtp is down") {
			t.Error("a raw error leaked into the response")
		}
	})
}

func TestCreateWithoutEmailConfiguredStillReturnsTheLink(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, mail.Unconfigured{})
		created, err := f.svc.Create(context.Background(), f.root, CreateInput{Email: "kim@example.com", SendEmail: true})
		if err != nil {
			t.Fatal(err)
		}
		tokenOf(t, created.Link)
		if created.EmailAvailable || created.EmailSent || created.EmailError != "" {
			t.Errorf("created = %+v", created)
		}
	})
}

func TestCreateRefusesAnExistingAccount(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		f.user("sam@example.com")
		_, err := f.svc.Create(context.Background(), f.root, CreateInput{Email: "SAM@example.com"})
		if !errors.Is(err, ErrEmailRegistered) {
			t.Fatalf("error = %v, want ErrEmailRegistered", err)
		}
	})
}

func TestCreateValidatesInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ws := f.workspace(f.root, "Acme Ltd", "acme")
		ctx := context.Background()
		var authInvalid *auth.ValidationError
		if _, err := f.svc.Create(ctx, f.root, CreateInput{Email: "nope"}); !errors.As(err, &authInvalid) {
			t.Errorf("bad email error = %v", err)
		}
		var invalid *validate.Error
		if _, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Workspace: ws.Slug, Role: "owner"}); !errors.As(err, &invalid) {
			t.Errorf("bad role error = %v", err)
		}
		if _, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Role: "write"}); !errors.As(err, &invalid) {
			t.Errorf("role without workspace error = %v", err)
		}
		if _, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Workspace: "missing"}); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("unknown workspace error = %v", err)
		}
		personal, err := f.ws.Personal(ctx, f.root.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Workspace: personal.Workspace.Slug}); !errors.Is(err, workspaces.ErrPersonalWorkspace) {
			t.Errorf("personal workspace error = %v", err)
		}
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Workspace: ws.Slug})
		if err != nil || created.Invite.Role != workspaces.RoleRead {
			t.Errorf("default role = %v, %v; want read", created.Invite.Role, err)
		}
	})
}

func TestWhoCanInviteMatrix(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		admin := f.user("admin@example.com")
		writer := f.user("writer@example.com")
		outsider := f.user("outsider@example.com")
		f.workspace(admin, "Acme Ltd", "acme")
		f.workspace(f.root, "Beta Ltd", "beta")
		if _, err := f.ws.AddMember(ctx, admin.ID, "acme", workspaces.AddMemberInput{Email: writer.Email, Role: "write"}); err != nil {
			t.Fatal(err)
		}
		invite := func(actor auth.User, workspace string) error {
			_, err := f.svc.Create(ctx, actor, CreateInput{Email: "new" + actor.ID[:6] + workspace + "@example.com", Workspace: workspace})
			return err
		}

		for _, actor := range []auth.User{admin, writer, outsider} {
			if err := invite(actor, "acme"); !errors.Is(err, ErrNotPermitted) {
				t.Errorf("root-only policy: %s error = %v, want ErrNotPermitted", actor.Email, err)
			}
		}
		if err := invite(f.root, ""); err != nil {
			t.Errorf("root-only policy: root without workspace: %v", err)
		}
		if err := invite(f.root, "acme"); err != nil {
			t.Errorf("root-only policy: root into any workspace: %v", err)
		}

		f.setPolicy("workspaceAdmins")
		cases := []struct {
			name      string
			actor     auth.User
			workspace string
			want      error
		}{
			{"admin into own workspace", admin, "acme", nil},
			{"admin without a workspace", admin, "", ErrNotPermitted},
			{"admin into a workspace they cannot see", admin, "beta", workspaces.ErrNotFound},
			{"writer into the workspace", writer, "acme", workspaces.ErrForbidden},
			{"outsider into the workspace", outsider, "acme", workspaces.ErrNotFound},
			{"root without a workspace", f.root, "", nil},
			{"root into any workspace", f.root, "beta", nil},
		}
		for _, tc := range cases {
			err := invite(tc.actor, tc.workspace)
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("workspaceAdmins policy: %s: error = %v, want %v", tc.name, err, tc.want)
			}
		}
	})
}
