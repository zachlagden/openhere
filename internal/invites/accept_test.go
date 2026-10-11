package invites

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/auth"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func TestAcceptCreatesTheAccountWorkspaceAndMembership(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		ws := f.workspace(f.root, "Acme Ltd", "acme")
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Workspace: ws.Slug, Role: "write"})
		if err != nil {
			t.Fatal(err)
		}
		token := tokenOf(t, created.Link)
		preview, err := f.svc.Preview(ctx, token)
		if err != nil || preview.Email != "kim@example.com" || preview.WorkspaceName != "Acme Ltd" {
			t.Fatalf("preview = %+v, %v", preview, err)
		}
		user, err := f.svc.Accept(ctx, token, testPassword, " Kim Example ")
		if err != nil {
			t.Fatal(err)
		}
		if user.Email != "kim@example.com" || user.IsRoot || user.DisplayName != "Kim Example" {
			t.Errorf("user = %+v", user)
		}
		if _, err := f.auth.Authenticate(ctx, "kim@example.com", testPassword); err != nil {
			t.Errorf("the new user cannot log in: %v", err)
		}
		if _, err := f.ws.Personal(ctx, user.ID); err != nil {
			t.Errorf("no personal workspace: %v", err)
		}
		acc, err := f.ws.Access(ctx, user.ID, "acme")
		if err != nil || acc.WorkspaceRole() != workspaces.RoleWrite {
			t.Errorf("membership = %v, %v; want write", acc.WorkspaceRole(), err)
		}
		if _, err := f.svc.Preview(ctx, token); !errors.Is(err, ErrInviteInvalid) {
			t.Errorf("a used invite previews as %v", err)
		}
	})
}

func TestAcceptWithoutAWorkspaceOnlyCreatesTheAccount(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		user, err := f.svc.Accept(ctx, tokenOf(t, created.Link), testPassword, "")
		if err != nil {
			t.Fatal(err)
		}
		list, err := f.ws.List(ctx, user.ID)
		if err != nil || len(list) != 1 || list[0].Workspace.Kind != workspaces.KindPersonal {
			t.Fatalf("workspaces = %+v, %v", list, err)
		}
	})
}

func TestAcceptIsSingleUse(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		token := tokenOf(t, created.Link)
		if _, err := f.svc.Accept(ctx, token, testPassword, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Accept(ctx, token, "another long password", ""); !errors.Is(err, ErrInviteInvalid) {
			t.Fatalf("second accept error = %v, want ErrInviteInvalid", err)
		}
		if _, err := f.auth.Authenticate(ctx, "kim@example.com", "another long password"); err == nil {
			t.Error("the second accept changed the password")
		}
	})
}

func TestUnusableInvitesAllFailTheSameWay(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		expired, err := f.svc.Create(ctx, f.root, CreateInput{Email: "old@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(Lifetime + time.Second)
		revoked, err := f.svc.Create(ctx, f.root, CreateInput{Email: "gone@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Revoke(ctx, f.root, revoked.Invite.ID); err != nil {
			t.Fatal(err)
		}
		used, err := f.svc.Create(ctx, f.root, CreateInput{Email: "done@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Accept(ctx, tokenOf(t, used.Link), testPassword, ""); err != nil {
			t.Fatal(err)
		}
		for name, token := range map[string]string{
			"expired": tokenOf(t, expired.Link), "revoked": tokenOf(t, revoked.Link), "used": tokenOf(t, used.Link),
			"unknown": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "short": "abc", "empty": "",
		} {
			if _, err := f.svc.Preview(ctx, token); !errors.Is(err, ErrInviteInvalid) {
				t.Errorf("%s preview error = %v, want ErrInviteInvalid", name, err)
			}
			if _, err := f.svc.Accept(ctx, token, testPassword, ""); !errors.Is(err, ErrInviteInvalid) {
				t.Errorf("%s accept error = %v, want ErrInviteInvalid", name, err)
			}
		}
		if _, err := f.auth.UserByEmail(ctx, "old@example.com"); !errors.Is(err, auth.ErrUserNotFound) {
			t.Error("an expired invite created an account")
		}
	})
}

func TestAcceptValidatesThePasswordWithoutSpendingTheInvite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		token := tokenOf(t, created.Link)
		var invalid *auth.ValidationError
		if _, err := f.svc.Accept(ctx, token, "short", ""); !errors.As(err, &invalid) {
			t.Fatalf("short password error = %v", err)
		}
		if _, err := f.svc.Accept(ctx, token, testPassword, "bad\nname"); err == nil {
			t.Fatal("a display name with a line break was accepted")
		}
		if _, err := f.svc.Accept(ctx, token, testPassword, ""); err != nil {
			t.Fatalf("the invite was spent by a rejected attempt: %v", err)
		}
	})
}

func TestAcceptFailsWhenTheEmailGotAnAccountInTheMeantime(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		f.user("kim@example.com")
		if _, err := f.svc.Accept(ctx, tokenOf(t, created.Link), testPassword, ""); !errors.Is(err, ErrInviteInvalid) {
			t.Fatalf("error = %v, want ErrInviteInvalid", err)
		}
		inv, err := f.svc.visible(ctx, f.root, created.Invite.ID)
		if err != nil || inv.AcceptedAt != nil {
			t.Errorf("a failed accept marked the invite accepted: %+v, %v", inv, err)
		}
	})
}

func TestConcurrentAcceptHasOneWinner(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		token := tokenOf(t, created.Link)
		results := make(chan error, 4)
		for range 4 {
			go func() {
				_, err := f.svc.Accept(ctx, token, testPassword, "")
				results <- err
			}()
		}
		wins := 0
		for range 4 {
			if err := <-results; err == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("%d accepts succeeded, want exactly 1", wins)
		}
	})
}
