package sites

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/slug"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestCheckSlug(t *testing.T) {
	valid := []string{"docs-site", "a", "launch-2026", "my--site", strings.Repeat("a", 63)}
	invalid := []string{
		"", "Docs", "docs_site", "-docs", "docs-", "docs.site", "docs site", strings.Repeat("a", 64),
		"www", "api", "app", "admin", "dashboard", "mail", "smtp", "static", "assets", "s", "docs", "status", "login", "setup", "settings",
		"0123456789abcdef0123456789abcdef",
	}
	for _, s := range valid {
		if err := CheckSlug(s); err != nil {
			t.Errorf("CheckSlug(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range invalid {
		if err := CheckSlug(s); !isValidation(err) {
			t.Errorf("CheckSlug(%q) = %v, want validation error", s, err)
		}
	}
}

func TestRandomSlugIsValid(t *testing.T) {
	seen := map[string]bool{}
	for range 500 {
		generated, err := RandomSlug()
		if err != nil {
			t.Fatalf("RandomSlug: %v", err)
		}
		if err := slug.Check(generated); err != nil || IsReserved(generated) {
			t.Fatalf("generated slug %q is not usable: %v", generated, err)
		}
		if parts := strings.Split(generated, "-"); len(parts) != 3 || len(parts[2]) != suffixLength {
			t.Fatalf("generated slug %q has the wrong shape", generated)
		}
		seen[generated] = true
	}
	if len(seen) < 495 {
		t.Errorf("only %d distinct slugs in 500 draws", len(seen))
	}
}

func TestCreateSiteRejectsBadSlugs(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		for _, bad := range []string{"WWW", "api", "Not Valid", "-x", "x-"} {
			_, err := e.svc.CreateSite(context.Background(), CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: bad})
			if !isValidation(err) {
				t.Errorf("CreateSite(%q) = %v, want validation error", bad, err)
			}
		}
	})
}

func TestCreateSiteWithExplicitSlug(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		ctx := context.Background()
		site, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "launch-page", Description: "  Spring launch  "})
		if err != nil {
			t.Fatalf("CreateSite: %v", err)
		}
		if site.Slug != "launch-page" || site.DisplayName != "launch-page" || site.Description != "Spring launch" ||
			site.WorkspaceSlug != "acme" || site.FolderID != "" || site.CreatedBy != e.alex || site.Role != "admin" {
			t.Errorf("site = %+v", site)
		}
		_, err = e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "launch-page"})
		if !errors.Is(err, ErrSlugTaken) {
			t.Errorf("duplicate slug = %v, want ErrSlugTaken", err)
		}
		other, err := e.ws.CreateShared(ctx, e.sam, "Sam team", "")
		if err != nil {
			t.Fatalf("CreateShared: %v", err)
		}
		_, err = e.svc.CreateSite(ctx, CreateInput{UserID: e.sam, Workspace: other.Workspace.Slug, Slug: "launch-page"})
		if !errors.Is(err, ErrSlugTaken) {
			t.Errorf("slug taken in another workspace = %v, want ErrSlugTaken", err)
		}
	})
}

func TestGeneratedSlugRetriesOnCollision(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		queue := []string{"taken-one", "taken-one", "fresh-two"}
		calls := 0
		e := newEnv(t, db, Options{SlugGenerator: func() (string, error) {
			calls++
			next := queue[0]
			queue = queue[1:]
			return next, nil
		}})
		ctx := context.Background()
		first, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug})
		if err != nil || first.Slug != "taken-one" || first.DisplayName != "taken-one" {
			t.Fatalf("first = %+v, %v", first, err)
		}
		second, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug, DisplayName: "Spring page"})
		if err != nil || second.Slug != "fresh-two" || second.DisplayName != "Spring page" {
			t.Fatalf("second = %+v, %v", second, err)
		}
		if calls != 3 {
			t.Errorf("generator calls = %d, want 3", calls)
		}
	})
}

func TestGeneratedSlugGivesUp(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{SlugGenerator: func() (string, error) { return "always-taken", nil }})
		ctx := context.Background()
		if _, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug}); err != nil {
			t.Fatalf("first CreateSite: %v", err)
		}
		_, err := e.svc.CreateSite(ctx, CreateInput{UserID: e.alex, Workspace: e.acme.Slug})
		if !errors.Is(err, ErrSlugGeneration) {
			t.Errorf("exhausted retries = %v, want ErrSlugGeneration", err)
		}
	})
}
