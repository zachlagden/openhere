package invites

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

func TestListShowsRootEverythingAndOthersTheirOwn(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		admin := f.user("admin@example.com")
		other := f.user("other@example.com")
		f.workspace(admin, "Acme Ltd", "acme")
		f.setPolicy("workspaceAdmins")
		if _, err := f.svc.Create(ctx, f.root, CreateInput{Email: "one@example.com"}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Create(ctx, admin, CreateInput{Email: "two@example.com", Workspace: "acme", Role: "read"}); err != nil {
			t.Fatal(err)
		}
		all, err := f.svc.List(ctx, f.root)
		if err != nil || len(all) != 2 {
			t.Fatalf("root list = %d, %v; want 2", len(all), err)
		}
		own, err := f.svc.List(ctx, admin)
		if err != nil || len(own) != 1 || own[0].Email != "two@example.com" || own[0].WorkspaceSlug != "acme" {
			t.Fatalf("admin list = %+v, %v", own, err)
		}
		none, err := f.svc.List(ctx, other)
		if err != nil || len(none) != 0 {
			t.Fatalf("other list = %+v, %v", none, err)
		}
	})
}

func TestRevokeRules(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		admin := f.user("admin@example.com")
		other := f.user("other@example.com")
		f.workspace(admin, "Acme Ltd", "acme")
		f.setPolicy("workspaceAdmins")
		mine, err := f.svc.Create(ctx, admin, CreateInput{Email: "two@example.com", Workspace: "acme"})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Revoke(ctx, other, mine.Invite.ID); !errors.Is(err, ErrInviteNotFound) {
			t.Errorf("a stranger revoking got %v, want ErrInviteNotFound", err)
		}
		if err := f.svc.Revoke(ctx, admin, mine.Invite.ID); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Revoke(ctx, admin, mine.Invite.ID); err != nil {
			t.Errorf("revoking twice failed: %v", err)
		}
		if _, err := f.svc.Preview(ctx, tokenOf(t, mine.Link)); !errors.Is(err, ErrInviteInvalid) {
			t.Error("a revoked invite still previews")
		}
		if err := f.svc.Revoke(ctx, f.root, "no-such-invite"); !errors.Is(err, ErrInviteNotFound) {
			t.Errorf("unknown invite error = %v", err)
		}
		used, err := f.svc.Create(ctx, f.root, CreateInput{Email: "three@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Accept(ctx, tokenOf(t, used.Link), testPassword, ""); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Revoke(ctx, f.root, used.Invite.ID); !errors.Is(err, ErrAccepted) {
			t.Errorf("revoking an accepted invite got %v, want ErrAccepted", err)
		}
	})
}

func TestResendIssuesANewLinkAndKillsTheOldOne(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		ws := f.workspace(f.root, "Acme Ltd", "acme")
		first, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com", Workspace: ws.Slug, Role: "admin"})
		if err != nil {
			t.Fatal(err)
		}
		second, err := f.svc.Resend(ctx, f.root, first.Invite.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if second.Link == first.Link || second.Invite.ID == first.Invite.ID || !second.EmailSent {
			t.Fatalf("resend = %+v", second)
		}
		if second.Invite.Role != workspaces.RoleAdmin || second.Invite.WorkspaceSlug != "acme" {
			t.Errorf("resend lost the workspace or role: %+v", second.Invite)
		}
		if _, err := f.svc.Preview(ctx, tokenOf(t, first.Link)); !errors.Is(err, ErrInviteInvalid) {
			t.Error("the old link still works after a resend")
		}
		if _, err := f.svc.Preview(ctx, tokenOf(t, second.Link)); err != nil {
			t.Errorf("the new link does not work: %v", err)
		}
		if _, err := f.svc.Accept(ctx, tokenOf(t, second.Link), testPassword, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Resend(ctx, f.root, second.Invite.ID, false); !errors.Is(err, ErrAccepted) {
			t.Errorf("resending an accepted invite got %v, want ErrAccepted", err)
		}
	})
}

func TestResendOfAnExpiredInviteWorksAndRespectsThePolicy(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		admin := f.user("admin@example.com")
		other := f.user("other@example.com")
		f.workspace(admin, "Acme Ltd", "acme")
		f.setPolicy("workspaceAdmins")
		created, err := f.svc.Create(ctx, admin, CreateInput{Email: "kim@example.com", Workspace: "acme"})
		if err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(Lifetime + time.Hour)
		if _, err := f.svc.Resend(ctx, other, created.Invite.ID, false); !errors.Is(err, ErrInviteNotFound) {
			t.Errorf("a stranger resending got %v, want ErrInviteNotFound", err)
		}
		fresh, err := f.svc.Resend(ctx, admin, created.Invite.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Preview(ctx, tokenOf(t, fresh.Link)); err != nil {
			t.Errorf("the resent link does not work: %v", err)
		}
		f.setPolicy("root")
		if _, err := f.svc.Resend(ctx, admin, fresh.Invite.ID, false); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("resending under a root-only policy got %v, want ErrNotPermitted", err)
		}
	})
}

func TestResendRefusesWhenTheEmailNowHasAnAccount(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		f.user("kim@example.com")
		if _, err := f.svc.Resend(ctx, f.root, created.Invite.ID, false); !errors.Is(err, ErrEmailRegistered) {
			t.Errorf("error = %v, want ErrEmailRegistered", err)
		}
	})
}

func TestStatusReflectsTheLifecycle(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db, nil)
		ctx := context.Background()
		created, err := f.svc.Create(ctx, f.root, CreateInput{Email: "kim@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		statusOf := func() string {
			list, err := f.svc.List(ctx, f.root)
			if err != nil || len(list) == 0 {
				t.Fatalf("list = %+v, %v", list, err)
			}
			return list[0].Status(f.clock.Now())
		}
		if got := statusOf(); got != StatusPending {
			t.Errorf("status = %s, want pending", got)
		}
		f.clock.Advance(Lifetime + time.Second)
		if got := statusOf(); got != StatusExpired {
			t.Errorf("status = %s, want expired", got)
		}
		if err := f.svc.Revoke(ctx, f.root, created.Invite.ID); err != nil {
			t.Fatal(err)
		}
		if got := statusOf(); got != StatusRevoked {
			t.Errorf("status = %s, want revoked", got)
		}
	})
}
