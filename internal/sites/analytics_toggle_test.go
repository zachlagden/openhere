package sites

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func TestNewSitesCountVisitsWithoutAHook(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		if site := e.site("counted", ""); !site.AnalyticsEnabled {
			t.Error("a new site has analytics off")
		}
	})
}

func TestNewSitesTakeTheConfiguredAnalyticsDefault(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		enabled := false
		e := newEnv(t, db, Options{AnalyticsDefault: func(context.Context) (bool, error) { return enabled, nil }})
		if off := e.site("quiet", ""); off.AnalyticsEnabled {
			t.Error("the site ignored a default of off")
		}
		enabled = true
		if on := e.site("loud", ""); !on.AnalyticsEnabled {
			t.Error("the site ignored a default of on")
		}
		got, err := e.svc.GetSite(context.Background(), e.alex, "quiet")
		if err != nil || got.AnalyticsEnabled {
			t.Errorf("loaded AnalyticsEnabled = %v, err = %v", got.AnalyticsEnabled, err)
		}
	})
}

func TestCreateSiteFailsWhenTheAnalyticsDefaultCannotBeRead(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		boom := errors.New("settings unavailable")
		e := newEnv(t, db, Options{AnalyticsDefault: func(context.Context) (bool, error) { return false, boom }})
		_, err := e.svc.CreateSite(context.Background(), CreateInput{UserID: e.alex, Workspace: e.acme.Slug, Slug: "never"})
		if !errors.Is(err, boom) {
			t.Fatalf("error = %v, want the settings error", err)
		}
	})
}

func TestUpdateSiteTogglesAnalytics(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		e.site("toggle", "")
		off := false
		updated, err := e.svc.UpdateSite(context.Background(), e.alex, "toggle", UpdateInput{Analytics: &off})
		if err != nil || updated.AnalyticsEnabled {
			t.Fatalf("UpdateSite = %+v, %v", updated, err)
		}
		other, err := e.svc.UpdateSite(context.Background(), e.alex, "toggle", UpdateInput{DisplayName: ptr("Renamed")})
		if err != nil || other.AnalyticsEnabled {
			t.Fatalf("an unrelated update changed the toggle: %+v, %v", other, err)
		}
		on := true
		again, err := e.svc.UpdateSite(context.Background(), e.alex, "toggle", UpdateInput{Analytics: &on})
		if err != nil || !again.AnalyticsEnabled {
			t.Fatalf("UpdateSite = %+v, %v", again, err)
		}
	})
}

func TestToggleNeedsWriteAccess(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db, Options{})
		e.site("guarded", "")
		e.grant("sam@example.com", "read", "workspace", "")
		off := false
		_, err := e.svc.UpdateSite(context.Background(), e.sam, "guarded", UpdateInput{Analytics: &off})
		if !errors.Is(err, workspaces.ErrForbidden) {
			t.Fatalf("error = %v, want forbidden", err)
		}
	})
}

func ptr[T any](v T) *T { return &v }
