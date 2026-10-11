package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
	"github.com/zachlagden/openhere/internal/workspaces"
)

func patchJSON(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *env) grant(email, role, scopeType, scopeID string) {
	e.t.Helper()
	in := workspaces.AddMemberInput{Email: email, Role: role, ScopeType: scopeType, ScopeID: scopeID}
	if _, err := e.ws.AddMember(context.Background(), e.alex, e.acme.Slug, in); err != nil {
		e.t.Fatal(err)
	}
}

func TestSiteReportPermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		site := e.site("demo")
		e.insert(fixedData(site.ID)...)
		ctx := context.Background()
		params := rangeParams("2026-03-08", "2026-03-10")

		if _, err := e.svc.SiteReport(ctx, e.sam, "demo", params); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("a stranger got %v, want not found", err)
		}
		if _, err := e.svc.SiteReport(ctx, e.alex, "missing", params); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("an unknown site gave %v, want not found", err)
		}
		e.grant("sam@example.com", "read", "site", site.ID)
		report, err := e.svc.SiteReport(ctx, e.sam, "demo", params)
		if err != nil || report.Totals.Views != 8 {
			t.Errorf("a site reader got %+v, %v", report.Totals, err)
		}
	})
}

func TestWorkspaceReportPermissionsAndAggregation(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		one, two := e.site("one"), e.site("two")
		e.insert(fixedData(one.ID)...)
		e.insert(view(two.ID, "2026-03-08T10:00:00Z", "/two", "t1"), view(two.ID, "2026-03-09T10:00:00Z", "/two", "t2"))
		other, err := e.ws.CreateShared(context.Background(), e.alex, "Globex Ltd", "globex")
		if err != nil {
			t.Fatal(err)
		}
		elsewhere, err := e.sites.CreateSite(context.Background(), siteInput(e.alex, other.Workspace.Slug, "elsewhere"))
		if err != nil {
			t.Fatal(err)
		}
		e.insert(view(elsewhere.ID, "2026-03-08T10:00:00Z", "/elsewhere", "x1"))
		ctx := context.Background()
		params := rangeParams("2026-03-08", "2026-03-10")

		if _, err := e.svc.WorkspaceReport(ctx, e.sam, "acme", params); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("a stranger got %v, want not found", err)
		}
		e.grant("sam@example.com", "read", "site", one.ID)
		if _, err := e.svc.WorkspaceReport(ctx, e.sam, "acme", params); !errors.Is(err, workspaces.ErrForbidden) {
			t.Errorf("a user with one site grant got %v, want forbidden", err)
		}
		report, err := e.svc.WorkspaceReport(ctx, e.alex, "acme", params)
		if err != nil {
			t.Fatal(err)
		}
		if report.Totals.Views != 10 {
			t.Errorf("workspace views = %d, want the two acme sites only (10)", report.Totals.Views)
		}
		if _, hasElsewhere := valuesOf(report.Paths)["/elsewhere"]; hasElsewhere {
			t.Error("the workspace report includes a site from another workspace")
		}
		e.grant("sam@example.com", "read", "workspace", "")
		wide, err := e.svc.WorkspaceReport(ctx, e.sam, "acme", params)
		if err != nil || wide.Totals.Views != 10 {
			t.Errorf("a workspace reader got %+v, %v", wide.Totals, err)
		}
		instance, err := e.svc.InstanceReport(ctx, params)
		if err != nil || instance.Totals.Views != 11 {
			t.Errorf("instance totals = %+v, %v", instance.Totals, err)
		}
	})
}

func TestPersonalWorkspaceReport(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		personal, err := e.ws.Personal(context.Background(), e.alex)
		if err != nil {
			t.Fatal(err)
		}
		mine, err := e.sites.CreateSite(context.Background(), siteInput(e.alex, personal.Workspace.Slug, "mine"))
		if err != nil {
			t.Fatal(err)
		}
		e.insert(view(mine.ID, "2026-03-08T10:00:00Z", "/mine", "m1"))
		e.insert(fixedData(e.site("shared").ID)...)
		params := rangeParams("2026-03-08", "2026-03-10")
		report, err := e.svc.WorkspaceReport(context.Background(), e.alex, personal.Workspace.Slug, params)
		if err != nil || report.Totals.Views != 1 {
			t.Errorf("personal report = %+v, %v", report.Totals, err)
		}
		if _, err := e.svc.WorkspaceReport(context.Background(), e.sam, personal.Workspace.Slug, params); !errors.Is(err, workspaces.ErrNotFound) {
			t.Errorf("another user read a personal workspace: %v", err)
		}
	})
}
