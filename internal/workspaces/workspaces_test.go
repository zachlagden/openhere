package workspaces

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/validate"
)

func TestPersonalWorkspace(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		ctx := context.Background()
		alex := f.user("alex@example.com")

		list, err := f.svc.List(ctx, alex)
		if err != nil || len(list) != 1 {
			t.Fatalf("List = %v, %v; want one workspace", list, err)
		}
		ws := list[0]
		if ws.Workspace.Kind != KindPersonal || ws.WorkspaceRole() != RoleAdmin || ws.ScopedOnly() {
			t.Errorf("personal workspace = %+v", ws)
		}
		if ws.Workspace.Slug != "personal-"+alex[:12] {
			t.Errorf("slug = %q", ws.Workspace.Slug)
		}

		members, err := f.svc.ListMembers(ctx, alex, ws.Workspace.Slug)
		if err != nil || len(members) != 1 || members[0].UserID != alex || members[0].Role != RoleAdmin {
			t.Fatalf("members = %+v, %v", members, err)
		}

		if err := f.svc.Delete(ctx, alex, ws.Workspace.Slug); !errors.Is(err, ErrPersonalWorkspace) {
			t.Errorf("Delete personal = %v, want ErrPersonalWorkspace", err)
		}
		sam := f.user("sam@example.com")
		_, err = f.svc.AddMember(ctx, alex, ws.Workspace.ID, AddMemberInput{Email: "sam@example.com", Role: "read"})
		if !errors.Is(err, ErrPersonalWorkspace) {
			t.Errorf("AddMember to personal = %v, want ErrPersonalWorkspace", err)
		}
		if _, err := f.svc.Access(ctx, sam, ws.Workspace.Slug); !errors.Is(err, ErrNotFound) {
			t.Errorf("another user's access to a personal workspace = %v, want ErrNotFound", err)
		}
		renamed, err := f.svc.Rename(ctx, alex, ws.Workspace.Slug, "Alex home")
		if err != nil || renamed.Workspace.Name != "Alex home" {
			t.Errorf("Rename personal = %+v, %v", renamed, err)
		}
	})
}

func TestPersonalWorkspaceIsUniquePerUser(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		alex := f.user("alex@example.com")
		if _, err := CreatePersonal(context.Background(), db, alex, testNow); !store.IsUniqueViolation(err) {
			t.Errorf("second personal workspace error = %v, want a unique violation", err)
		}
	})
}

func TestCreateShared(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		ctx := context.Background()
		alex := f.user("alex@example.com")

		acc, err := f.svc.CreateShared(ctx, alex, "Acme Ltd", "")
		if err != nil {
			t.Fatalf("CreateShared: %v", err)
		}
		if acc.Workspace.Slug != "acme-ltd" || acc.Workspace.Kind != KindShared || acc.WorkspaceRole() != RoleAdmin {
			t.Errorf("workspace = %+v role %s", acc.Workspace, acc.WorkspaceRole())
		}

		second, err := f.svc.CreateShared(ctx, alex, "Acme Ltd", "")
		if err != nil || !strings.HasPrefix(second.Workspace.Slug, "acme-ltd-") {
			t.Errorf("derived slug on collision = %q, %v", second.Workspace.Slug, err)
		}

		named, err := f.svc.CreateShared(ctx, alex, "Docs", "team-docs")
		if err != nil || named.Workspace.Slug != "team-docs" {
			t.Fatalf("explicit slug = %+v, %v", named, err)
		}
		if _, err := f.svc.CreateShared(ctx, alex, "Other", "team-docs"); !errors.Is(err, ErrSlugTaken) {
			t.Errorf("duplicate explicit slug = %v, want ErrSlugTaken", err)
		}
		fallback, err := f.svc.CreateShared(ctx, alex, "!!!", "")
		if err != nil || fallback.Workspace.Slug != "workspace" {
			t.Errorf("fallback slug = %+v, %v", fallback, err)
		}
	})
}

func TestCreateSharedValidation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		alex := f.user("alex@example.com")
		cases := []struct{ name, slug string }{
			{"", ""}, {"   ", ""}, {strings.Repeat("a", 65), ""}, {"Bad\x00name", ""},
			{"Ok", "Bad_Slug"}, {"Ok", "-lead"}, {"Ok", "0123456789abcdef0123456789abcdef"},
		}
		for _, c := range cases {
			_, err := f.svc.CreateShared(context.Background(), alex, c.name, c.slug)
			var invalid *validate.Error
			if !errors.As(err, &invalid) {
				t.Errorf("CreateShared(%q, %q) = %v, want validation error", c.name, c.slug, err)
			}
		}
	})
}

func TestCreateSharedCanBeGated(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		denied := errors.New("shared workspaces are disabled")
		svc := New(db, Options{MayCreateShared: func(context.Context, string) error { return denied }})
		alex := storetest.InsertUser(t, db, "alex@example.com")
		if _, err := svc.CreateShared(context.Background(), alex, "Acme Ltd", ""); !errors.Is(err, denied) {
			t.Errorf("CreateShared = %v, want the gate error", err)
		}
	})
}

func TestListShowsScopedOnlyWorkspaces(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		ctx := context.Background()
		alex := f.user("alex@example.com")
		sam := f.user("sam@example.com")
		acme := f.shared(alex, "Acme Ltd")
		folder := f.folder(acme.Workspace.ID, "Reports")
		f.addMember(alex, acme.Workspace.Slug, "sam@example.com", "read", "folder", folder)

		list, err := f.svc.List(ctx, sam)
		if err != nil || len(list) != 2 {
			t.Fatalf("List = %d workspaces, %v; want personal and shared", len(list), err)
		}
		if list[0].Workspace.Kind != KindPersonal || list[1].Workspace.ID != acme.Workspace.ID {
			t.Errorf("order = %s, %s", list[0].Workspace.Kind, list[1].Workspace.Slug)
		}
		if !list[1].ScopedOnly() || list[1].WorkspaceRole() != RoleRead {
			t.Errorf("scoped workspace = scopedOnly %v role %s", list[1].ScopedOnly(), list[1].WorkspaceRole())
		}
		if _, err := f.svc.Authorize(ctx, sam, acme.Workspace.Slug, RoleAdmin); !errors.Is(err, ErrForbidden) {
			t.Errorf("scoped reader as admin = %v, want ErrForbidden", err)
		}
	})
}

func TestAccessByIDOrSlug(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		ctx := context.Background()
		alex := f.user("alex@example.com")
		acme := f.shared(alex, "Acme Ltd")
		for _, ref := range []string{acme.Workspace.ID, acme.Workspace.Slug} {
			got, err := f.svc.Access(ctx, alex, ref)
			if err != nil || got.Workspace.ID != acme.Workspace.ID {
				t.Errorf("Access(%q) = %+v, %v", ref, got.Workspace, err)
			}
		}
		if _, err := f.svc.Access(ctx, alex, "missing"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Access(missing) = %v, want ErrNotFound", err)
		}
	})
}

func TestRenameAndDelete(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		f := newFixture(t, db)
		ctx := context.Background()
		alex := f.user("alex@example.com")
		sam := f.user("sam@example.com")
		acme := f.shared(alex, "Acme Ltd")
		f.addMember(alex, acme.Workspace.Slug, "sam@example.com", "write", "workspace", "")

		if _, err := f.svc.Rename(ctx, sam, acme.Workspace.Slug, "Mine"); !errors.Is(err, ErrForbidden) {
			t.Errorf("writer rename = %v, want ErrForbidden", err)
		}
		if err := f.svc.Delete(ctx, sam, acme.Workspace.Slug); !errors.Is(err, ErrForbidden) {
			t.Errorf("writer delete = %v, want ErrForbidden", err)
		}
		renamed, err := f.svc.Rename(ctx, alex, acme.Workspace.Slug, "Acme Group")
		if err != nil || renamed.Workspace.Name != "Acme Group" || renamed.Workspace.Slug != acme.Workspace.Slug {
			t.Errorf("Rename = %+v, %v", renamed.Workspace, err)
		}
		if _, err := f.svc.Rename(ctx, alex, acme.Workspace.Slug, " "); err == nil {
			t.Error("blank rename accepted")
		}

		site := f.site(acme.Workspace.ID, "launch", "")
		if err := f.svc.Delete(ctx, alex, acme.Workspace.Slug); !errors.Is(err, ErrNotEmpty) {
			t.Errorf("Delete with sites = %v, want ErrNotEmpty", err)
		}
		f.exec("DELETE FROM sites WHERE id = $1", site)
		f.folder(acme.Workspace.ID, "Reports")
		if err := f.svc.Delete(ctx, alex, acme.Workspace.Slug); err != nil {
			t.Fatalf("Delete empty workspace: %v", err)
		}
		if _, err := f.svc.Access(ctx, alex, acme.Workspace.Slug); !errors.Is(err, ErrNotFound) {
			t.Errorf("Access after delete = %v, want ErrNotFound", err)
		}
		var rows int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workspace_access WHERE workspace_id = $1", acme.Workspace.ID).Scan(&rows); err != nil || rows != 0 {
			t.Errorf("grants left after delete = %d, %v", rows, err)
		}
	})
}
