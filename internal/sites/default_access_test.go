package sites

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestNewSitesArePublicWithoutAHook(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		site := e.site("plain", "")
		if site.AccessMode != "public" {
			t.Errorf("created AccessMode = %q", site.AccessMode)
		}
		got, err := e.svc.GetSite(context.Background(), e.alex, "plain")
		if err != nil || got.AccessMode != "public" {
			t.Errorf("loaded AccessMode = %q, err = %v", got.AccessMode, err)
		}
	})
}

func TestNewSitesTakeTheConfiguredDefault(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		mode := "restricted"
		e := newEnv(t, db, Options{DefaultAccessMode: func(context.Context) (string, error) { return mode, nil }})
		first := e.site("private-one", "")
		if first.AccessMode != "restricted" {
			t.Errorf("created AccessMode = %q", first.AccessMode)
		}
		mode = "public"
		second := e.site("open-one", "")
		if second.AccessMode != "public" {
			t.Errorf("created AccessMode = %q", second.AccessMode)
		}
		page, err := e.svc.ListSites(context.Background(), e.alex, Filter{})
		if err != nil {
			t.Fatal(err)
		}
		modes := map[string]string{}
		for _, s := range page.Sites {
			modes[s.Slug] = s.AccessMode
		}
		if modes["private-one"] != "restricted" || modes["open-one"] != "public" {
			t.Errorf("listed modes = %v", modes)
		}
	})
}

func TestCreateSiteFailsWhenTheDefaultCannotBeRead(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		boom := errors.New("settings unavailable")
		e := newEnv(t, db, Options{DefaultAccessMode: func(context.Context) (string, error) { return "", boom }})
		_, err := e.svc.CreateSite(context.Background(), CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "never"})
		if !errors.Is(err, boom) {
			t.Fatalf("error = %v, want the settings error", err)
		}
	})
}
