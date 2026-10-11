package workspaces

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/validate"
)

type team struct {
	f      *fixture
	alex   string
	sam    string
	acme   Access
	slug   string
	folder string
	site   string
}

func newTeam(t *testing.T, db store.Store) team {
	t.Helper()
	f := newFixture(t, db)
	alex := f.user("alex@example.com")
	sam := f.user("sam@example.com")
	acme := f.shared(alex, "Acme Ltd")
	return team{
		f: f, alex: alex, sam: sam, acme: acme, slug: acme.Workspace.Slug,
		folder: f.folder(acme.Workspace.ID, "Reports"),
		site:   f.site(acme.Workspace.ID, "launch", ""),
	}
}

func TestAddMemberScopes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		ctx := context.Background()

		whole := tm.f.addMember(tm.alex, tm.slug, "SAM@example.com", "write", "", "")
		if whole.ScopeType != ScopeWorkspace || whole.Role != RoleWrite || whole.Email != "sam@example.com" {
			t.Errorf("workspace member = %+v", whole)
		}
		folder := tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "read", "folder", tm.folder)
		if folder.FolderID != tm.folder || folder.ScopeName != "Reports" {
			t.Errorf("folder member = %+v", folder)
		}
		site := tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "write", "site", "launch")
		if site.SiteID != tm.site || site.ScopeName != "launch" {
			t.Errorf("site member by slug = %+v", site)
		}

		members, err := tm.f.svc.ListMembers(ctx, tm.alex, tm.slug)
		if err != nil || len(members) != 4 {
			t.Fatalf("ListMembers = %d, %v; want owner plus three grants", len(members), err)
		}
		acc, err := tm.f.svc.Access(ctx, tm.sam, tm.slug)
		if err != nil || len(acc.Grants) != 3 {
			t.Fatalf("sam grants = %+v, %v", acc.Grants, err)
		}
	})
}

func TestAddMemberRejections(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		ctx := context.Background()
		add := func(in AddMemberInput) error {
			_, err := tm.f.svc.AddMember(ctx, tm.alex, tm.slug, in)
			return err
		}

		if err := add(AddMemberInput{Email: "nobody@example.com", Role: "read"}); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("unknown email = %v, want ErrUserNotFound", err)
		}
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "owner"}); !isValidation(err) {
			t.Errorf("bad role = %v, want validation error", err)
		}
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "admin", ScopeType: "folder", ScopeID: tm.folder}); !isValidation(err) {
			t.Errorf("scoped admin = %v, want validation error", err)
		}
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "read", ScopeType: "folder"}); !isValidation(err) {
			t.Errorf("folder without id = %v, want validation error", err)
		}
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "read", ScopeType: "team"}); !isValidation(err) {
			t.Errorf("unknown scope = %v, want validation error", err)
		}
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "read", ScopeID: tm.folder}); !isValidation(err) {
			t.Errorf("workspace scope with id = %v, want validation error", err)
		}
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "read", ScopeType: "folder", ScopeID: "missing"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("missing folder = %v, want ErrNotFound", err)
		}

		other := tm.f.shared(tm.sam, "Sam team")
		foreign := tm.f.folder(other.Workspace.ID, "Elsewhere")
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "read", ScopeType: "folder", ScopeID: foreign}); !errors.Is(err, ErrNotFound) {
			t.Errorf("folder of another workspace = %v, want ErrNotFound", err)
		}

		tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "read", "folder", tm.folder)
		if err := add(AddMemberInput{Email: "sam@example.com", Role: "write", ScopeType: "folder", ScopeID: tm.folder}); !errors.Is(err, ErrMemberExists) {
			t.Errorf("duplicate grant = %v, want ErrMemberExists", err)
		}
	})
}

func TestMemberManagementNeedsAdmin(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		ctx := context.Background()
		writer := tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "write", "workspace", "")

		if _, err := tm.f.svc.ListMembers(ctx, tm.sam, tm.slug); !errors.Is(err, ErrForbidden) {
			t.Errorf("writer ListMembers = %v, want ErrForbidden", err)
		}
		if _, err := tm.f.svc.AddMember(ctx, tm.sam, tm.slug, AddMemberInput{Email: "alex@example.com", Role: "read", ScopeType: "site", ScopeID: tm.site}); !errors.Is(err, ErrForbidden) {
			t.Errorf("writer AddMember = %v, want ErrForbidden", err)
		}
		if _, err := tm.f.svc.UpdateMember(ctx, tm.sam, tm.slug, writer.ID, "admin"); !errors.Is(err, ErrForbidden) {
			t.Errorf("writer self-promotion = %v, want ErrForbidden", err)
		}
		stranger := tm.f.user("kim@example.com")
		if _, err := tm.f.svc.ListMembers(ctx, stranger, tm.slug); !errors.Is(err, ErrNotFound) {
			t.Errorf("stranger ListMembers = %v, want ErrNotFound", err)
		}
	})
}

func TestLastAdminProtection(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		ctx := context.Background()
		members, err := tm.f.svc.ListMembers(ctx, tm.alex, tm.slug)
		if err != nil || len(members) != 1 {
			t.Fatalf("ListMembers = %+v, %v", members, err)
		}
		owner := members[0]

		if err := tm.f.svc.RemoveMember(ctx, tm.alex, tm.slug, owner.ID); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("remove last admin = %v, want ErrLastAdmin", err)
		}
		if _, err := tm.f.svc.UpdateMember(ctx, tm.alex, tm.slug, owner.ID, "write"); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("demote last admin = %v, want ErrLastAdmin", err)
		}

		second := tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "admin", "workspace", "")
		if _, err := tm.f.svc.UpdateMember(ctx, tm.alex, tm.slug, owner.ID, "write"); err != nil {
			t.Fatalf("demote with a second admin: %v", err)
		}
		if err := tm.f.svc.RemoveMember(ctx, tm.sam, tm.slug, second.ID); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("remove the remaining admin = %v, want ErrLastAdmin", err)
		}
		if err := tm.f.svc.RemoveMember(ctx, tm.sam, tm.slug, owner.ID); err != nil {
			t.Errorf("remove a non-admin: %v", err)
		}
	})
}

func TestScopedAdminsDoNotCountAndAdminCannotBeScoped(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		ctx := context.Background()
		scoped := tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "read", "folder", tm.folder)
		if _, err := tm.f.svc.UpdateMember(ctx, tm.alex, tm.slug, scoped.ID, "admin"); !isValidation(err) {
			t.Errorf("promote scoped grant to admin = %v, want validation error", err)
		}
		updated, err := tm.f.svc.UpdateMember(ctx, tm.alex, tm.slug, scoped.ID, "write")
		if err != nil || updated.Role != RoleWrite {
			t.Errorf("UpdateMember = %+v, %v", updated, err)
		}
		if _, err := tm.f.svc.UpdateMember(ctx, tm.alex, tm.slug, scoped.ID, "root"); !isValidation(err) {
			t.Errorf("unknown role = %v, want validation error", err)
		}
		if err := tm.f.svc.RemoveMember(ctx, tm.alex, tm.slug, scoped.ID); err != nil {
			t.Errorf("remove scoped grant: %v", err)
		}
		if err := tm.f.svc.RemoveMember(ctx, tm.alex, tm.slug, scoped.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("remove twice = %v, want ErrNotFound", err)
		}
	})
}

func TestGrantsDisappearWithTheirFolderAndSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		ctx := context.Background()
		tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "read", "folder", tm.folder)
		tm.f.addMember(tm.alex, tm.slug, "sam@example.com", "write", "site", tm.site)

		tm.f.exec("DELETE FROM folders WHERE id = $1", tm.folder)
		acc, err := tm.f.svc.Access(ctx, tm.sam, tm.slug)
		if err != nil || len(acc.Grants) != 1 || acc.Grants[0].ScopeType != ScopeSite {
			t.Fatalf("grants after folder delete = %+v, %v", acc.Grants, err)
		}
		tm.f.exec("DELETE FROM sites WHERE id = $1", tm.site)
		if _, err := tm.f.svc.Access(ctx, tm.sam, tm.slug); !errors.Is(err, ErrNotFound) {
			t.Errorf("access after the last grant went = %v, want ErrNotFound", err)
		}
	})
}

func TestConstraintsRejectInvalidGrantRows(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		tm := newTeam(t, db)
		insert := func(role, scope string, folder, site any) error {
			_, err := db.ExecContext(context.Background(),
				`INSERT INTO workspace_access (id, workspace_id, user_id, role, scope_type, folder_id, site_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				tm.f.newID(), tm.acme.Workspace.ID, tm.sam, role, scope, folder, site, testNow)
			return err
		}
		if err := insert("admin", "folder", tm.folder, nil); err == nil {
			t.Error("scoped admin row accepted")
		}
		if err := insert("read", "folder", nil, nil); err == nil {
			t.Error("folder scope without a folder accepted")
		}
		if err := insert("read", "workspace", tm.folder, nil); err == nil {
			t.Error("workspace scope with a folder accepted")
		}
		if err := insert("read", "folder", tm.folder, tm.site); err == nil {
			t.Error("row with both a folder and a site accepted")
		}
		if err := insert("owner", "workspace", nil, nil); err == nil {
			t.Error("unknown role accepted")
		}
		other := tm.f.shared(tm.sam, "Sam team")
		foreign := tm.f.folder(other.Workspace.ID, "Elsewhere")
		if err := insert("read", "folder", foreign, nil); err == nil {
			t.Error("folder from another workspace accepted")
		}
		if err := insert("read", "workspace", nil, nil); err != nil {
			t.Errorf("valid row rejected: %v", err)
		}
		if err := insert("write", "workspace", nil, nil); err == nil {
			t.Error("second workspace-wide row for one user accepted")
		}
	})
}

func isValidation(err error) bool {
	var invalid *validate.Error
	return errors.As(err, &invalid)
}
