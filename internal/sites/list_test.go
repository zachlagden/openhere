package sites

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/workspaces"
)

func TestListFilters(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		reports := e.folder("Reports")
		personal, err := e.ws.List(ctx, e.alex)
		if err != nil || len(personal) < 2 {
			t.Fatalf("List workspaces = %v, %v", personal, err)
		}
		home := personal[0].Workspace
		e.site("q1-report", reports.ID)
		e.site("launch", "")
		e.clock.Advance(time.Second)
		if _, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: home.Slug, Slug: "my-notes", DisplayName: "Reading Notes"}); err != nil {
			t.Fatalf("CreateSite personal: %v", err)
		}

		tests := []struct {
			name   string
			filter Filter
			want   []string
		}{
			{"everything", Filter{}, []string{"q1-report", "launch", "my-notes"}},
			{"one workspace", Filter{Workspace: "acme"}, []string{"q1-report", "launch"}},
			{"workspace by id", Filter{Workspace: home.ID}, []string{"my-notes"}},
			{"one folder", Filter{Folder: reports.ID}, []string{"q1-report"}},
			{"unfiled", Filter{Folder: FolderNone}, []string{"launch", "my-notes"}},
			{"unfiled in a workspace", Filter{Workspace: "acme", Folder: FolderNone}, []string{"launch"}},
			{"search slug", Filter{Search: "REPORT"}, []string{"q1-report"}},
			{"search display name", Filter{Search: "reading"}, []string{"my-notes"}},
			{"search with folder", Filter{Search: "l", Folder: FolderNone}, []string{"launch"}},
			{"search no match", Filter{Search: "zzz"}, nil},
			{"search wildcard is literal", Filter{Search: "%"}, nil},
			{"search underscore is literal", Filter{Search: "q_"}, nil},
			{"search blank", Filter{Search: "   "}, []string{"q1-report", "launch", "my-notes"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := e.slugs(e.alex, tt.filter); !sameSet(got, tt.want) {
					t.Errorf("slugs = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

func TestListOrdersNewestFirstAndPaginates(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		var created []string
		for i := range 25 {
			slug := fmt.Sprintf("site-%02d", i)
			if i%5 != 0 {
				e.clock.Advance(time.Second)
			}
			site, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: slug})
			if err != nil {
				t.Fatalf("CreateSite: %v", err)
			}
			created = append(created, site.Slug)
		}

		var collected []string
		filter := Filter{Limit: 7}
		pages := 0
		for {
			page, err := e.svc.ListSites(ctx, e.alex, filter)
			if err != nil {
				t.Fatalf("ListSites: %v", err)
			}
			pages++
			for _, site := range page.Sites {
				collected = append(collected, site.Slug)
			}
			if page.NextCursor == "" {
				break
			}
			filter.Cursor = page.NextCursor
		}
		if pages != 4 {
			t.Errorf("pages = %d, want 4", pages)
		}
		if len(collected) != 25 {
			t.Fatalf("collected %d sites, want 25: %v", len(collected), collected)
		}
		if !sameSet(collected, created) {
			t.Errorf("pagination lost or repeated sites: %v", collected)
		}
		whole, err := e.svc.ListSites(ctx, e.alex, Filter{Limit: 200})
		if err != nil {
			t.Fatalf("ListSites whole: %v", err)
		}
		for i, site := range whole.Sites {
			if site.Slug != collected[i] {
				t.Fatalf("paged order differs from the full listing at %d: %s vs %s", i, collected[i], site.Slug)
			}
		}
		for i := 1; i < len(whole.Sites); i++ {
			if whole.Sites[i-1].CreatedAt.Before(whole.Sites[i].CreatedAt) {
				t.Fatalf("not newest first at %d", i)
			}
		}
	})
}

func TestPaginationSurvivesInsertsBetweenPages(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		for i := range 6 {
			e.site(fmt.Sprintf("old-%d", i), "")
		}
		first, err := e.svc.ListSites(ctx, e.alex, Filter{Limit: 3})
		if err != nil || len(first.Sites) != 3 || first.NextCursor == "" {
			t.Fatalf("first page = %+v, %v", first, err)
		}
		e.site("newer", "")
		second, err := e.svc.ListSites(ctx, e.alex, Filter{Limit: 3, Cursor: first.NextCursor})
		if err != nil || len(second.Sites) != 3 || second.NextCursor != "" {
			t.Fatalf("second page = %+v, %v", second, err)
		}
		for _, site := range second.Sites {
			if site.Slug == "newer" {
				t.Error("a site created after the first page shifted the second page")
			}
		}
	})
}

func TestListRejectsBadPaging(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		for _, f := range []Filter{{Limit: -1}, {Limit: MaxLimit + 1}, {Cursor: "!!!"}, {Cursor: "bm9wZQ"}} {
			if _, err := e.svc.ListSites(context.Background(), e.alex, f); !isValidation(err) {
				t.Errorf("ListSites(%+v) = %v, want validation error", f, err)
			}
		}
	})
}

func TestListMatchesTheAuthorizationFunction(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newScopedSetup(t, db)
		ctx := context.Background()
		s.grant("sam@example.com", "read", "folder", s.reports.ID)
		s.grant("sam@example.com", "write", "site", s.unfiled.ID)
		s.grant("sam@example.com", "write", "site", s.inArchive.ID)
		s.grant("kim@example.com", "write", "folder", s.archive.ID)
		s.grant("kim@example.com", "read", "workspace", "")
		all := []Site{s.inReports, s.inArchive, s.unfiled}

		for _, userID := range []string{s.alex, s.sam, s.kim} {
			acc, err := s.ws.AccessByID(ctx, userID, s.acme.ID)
			if err != nil {
				t.Fatalf("AccessByID: %v", err)
			}
			want := map[string]workspaces.Role{}
			for _, site := range all {
				if role := acc.SiteRole(site.ID, site.FolderID); role != workspaces.RoleNone {
					want[site.Slug] = role
				}
			}
			page, err := s.svc.ListSites(ctx, userID, Filter{Workspace: s.acme.Slug})
			if err != nil {
				t.Fatalf("ListSites: %v", err)
			}
			got := map[string]workspaces.Role{}
			for _, site := range page.Sites {
				got[site.Slug] = site.Role
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("user %s: listed %v, authorization function says %v", userID[:6], got, want)
			}
			for _, site := range all {
				fetched, err := s.svc.GetSite(ctx, userID, site.Slug)
				wantRole, covered := want[site.Slug]
				if covered && (err != nil || fetched.Role != wantRole) {
					t.Errorf("GetSite(%s) = %+v, %v; want role %s", site.Slug, fetched, err, wantRole)
				}
				if !covered && err == nil {
					t.Errorf("GetSite(%s) succeeded for an uncovered user", site.Slug)
				}
			}
		}
	})
}

func TestSearchIsCaseInsensitiveOnDisplayName(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		e.clock.Advance(time.Second)
		_, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "plain-slug", DisplayName: "Annual REPORT 2026"})
		if err != nil {
			t.Fatalf("CreateSite: %v", err)
		}
		for _, term := range []string{"annual report", "ANNUAL", "report 20", strings.ToUpper("plain-")} {
			if got := e.slugs(e.alex, Filter{Search: term}); len(got) != 1 {
				t.Errorf("search %q = %v, want one match", term, got)
			}
		}
	})
}
