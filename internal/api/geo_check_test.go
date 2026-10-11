package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/analytics/geo"
	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

const geoCheckPath = "/api/v1/admin/analytics/geo/check"

type fakeGeo struct {
	h       *harness
	calls   int
	state   settings.GeoState
	tooSoon time.Duration
}

func (f *fakeGeo) ForceCheck(ctx context.Context) error {
	if f.tooSoon > 0 {
		return &geo.TooSoonError{RetryAfter: f.tooSoon}
	}
	f.calls++
	return f.h.settings.RecordGeo(ctx, f.state)
}

func (a actor) saveMaxMindKey() {
	a.h.t.Helper()
	a.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"analytics.maxmindAccountId":"123456","analytics.maxmindLicenseKey":"licence-example-key"}}`)
}

func TestGeoCheckIsRootOnlyAndNeverAnonymous(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		checker := &fakeGeo{state: settings.GeoState{Status: settings.GeoOK, Updated: true}}
		h := newHarness(t, db, harnessOptions{geo: checker})
		checker.h = h
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		root.saveMaxMindKey()

		expectError(t, h.do("POST", geoCheckPath, nil), http.StatusUnauthorized, codeUnauthenticated)
		sam.expectError(http.StatusNotFound, codeNotFound, "POST", geoCheckPath, nil)
		if checker.calls != 0 {
			t.Fatalf("a non-root caller ran %d checks", checker.calls)
		}
		root.expect(http.StatusOK, "POST", geoCheckPath, nil)
		if checker.calls != 1 {
			t.Errorf("calls = %d, want 1", checker.calls)
		}
	})
}

func TestGeoCheckIsAbsentWithoutAnUpdater(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.saveMaxMindKey()
		root.expectError(http.StatusNotFound, codeNotFound, "POST", geoCheckPath, nil)
	})
}

func TestGeoCheckNeedsASavedKey(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		checker := &fakeGeo{}
		h := newHarness(t, db, harnessOptions{geo: checker})
		checker.h = h
		root := h.rootActor()
		root.expectError(http.StatusConflict, codeGeoNotConfigured, "POST", geoCheckPath, nil)
		root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", `{"settings":{"analytics.maxmindAccountId":"123456"}}`)
		root.expectError(http.StatusConflict, codeGeoNotConfigured, "POST", geoCheckPath, nil)
		if checker.calls != 0 {
			t.Errorf("calls = %d, want 0", checker.calls)
		}
	})
}

func TestGeoCheckReportsTheOutcomeWithoutSecrets(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		checker := &fakeGeo{state: settings.GeoState{Status: settings.GeoFailed, Detail: "MaxMind rejected the account ID or licence key."}}
		h := newHarness(t, db, harnessOptions{geo: checker, logToBuf: true})
		checker.h = h
		root := h.rootActor()
		root.saveMaxMindKey()

		rec := root.call("POST", geoCheckPath, nil)
		expectStatus(t, rec, http.StatusOK)
		failed := decodeBody[geoCheckResponse](t, rec)
		if failed.Status != "failed" || failed.Detail != checker.state.Detail || failed.CheckedAt == nil || failed.UpdatedAt != nil {
			t.Errorf("failed result = %+v", failed)
		}

		checker.state = settings.GeoState{Status: settings.GeoOK, Updated: true}
		rec = root.call("POST", geoCheckPath, nil)
		ok := decodeBody[geoCheckResponse](t, rec)
		if ok.Status != "ok" || ok.Detail != "" || ok.CheckedAt == nil || ok.UpdatedAt == nil {
			t.Errorf("ok result = %+v", ok)
		}
		for name, text := range map[string]string{"response": rec.Body.String(), "log": h.logs.String()} {
			if strings.Contains(text, "licence-example-key") {
				t.Errorf("the %s holds the licence key", name)
			}
		}
	})
}

func TestGeoCheckAnswers429InsideTheFloor(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		checker := &fakeGeo{tooSoon: 42 * time.Second}
		h := newHarness(t, db, harnessOptions{geo: checker})
		checker.h = h
		root := h.rootActor()
		root.saveMaxMindKey()
		rec := root.call("POST", geoCheckPath, nil)
		expectError(t, rec, http.StatusTooManyRequests, codeRateLimited)
		if rec.Header().Get("Retry-After") != "42" {
			t.Errorf("Retry-After = %q, want 42", rec.Header().Get("Retry-After"))
		}
		body := decodeBody[errorBody](t, rec)
		if body.RetryAfter == nil || *body.RetryAfter != 42 {
			t.Errorf("retry_after = %v, want 42", body.RetryAfter)
		}
	})
}
