package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func scheduledFailure(t *testing.T, svc *Service) Settings {
	t.Helper()
	got, err := svc.RecordScheduledWildcard(context.Background(), false, "DNS lookup failed")
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestManualFailedCheckKeepsSubdomainModeOnWithoutCounting(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		setModes(t, svc, true, true)
		for range 5 {
			failed, err := svc.RecordWildcard(ctx, false, "DNS lookup failed")
			if err != nil {
				t.Fatal(err)
			}
			if !failed.SubdomainEnabled || failed.WildcardStatus != WildcardFailed || failed.WildcardDetail != "DNS lookup failed" {
				t.Fatalf("after a manual failure = %+v", failed)
			}
			if failed.WildcardFailures != 0 || failed.SubdomainDisabledReason != "" {
				t.Fatalf("a manual check was counted: %+v", failed)
			}
		}
	})
}

func TestSubdomainModeSwitchesOffAfterThreeScheduledFailures(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		setModes(t, svc, true, true)
		for want := 1; want <= 2; want++ {
			got := scheduledFailure(t, svc)
			if !got.SubdomainEnabled || got.WildcardFailures != want || got.SubdomainDisabledReason != "" {
				t.Fatalf("failure %d = %+v", want, got)
			}
		}
		third := scheduledFailure(t, svc)
		if third.SubdomainEnabled || !third.PathEnabled || third.WildcardFailures != 3 || third.SubdomainDisabledReason != DisabledByWildcard {
			t.Fatalf("third failure = %+v", third)
		}
		fourth := scheduledFailure(t, svc)
		if fourth.SubdomainEnabled || fourth.WildcardFailures != 3 {
			t.Errorf("a failure while the mode is off changed the counter: %+v", fourth)
		}
	})
}

func TestManualChecksBetweenScheduledFailuresDoNotCount(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		setModes(t, svc, true, true)
		scheduledFailure(t, svc)
		scheduledFailure(t, svc)
		for range 4 {
			if _, err := svc.RecordWildcard(ctx, false, "DNS lookup failed"); err != nil {
				t.Fatal(err)
			}
		}
		got, err := svc.Get(ctx)
		if err != nil || !got.SubdomainEnabled || got.WildcardFailures != 2 {
			t.Fatalf("after manual failures = %+v, %v", got, err)
		}
	})
}

func TestSubdomainModeStaysOnWhenItIsTheOnlyMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		setModes(t, svc, true, false)
		var got Settings
		for range 5 {
			got = scheduledFailure(t, svc)
		}
		if !got.SubdomainEnabled || got.WildcardFailures != 5 || got.SubdomainDisabledReason != "" {
			t.Errorf("after five failures with path mode off = %+v", got)
		}
	})
}

func TestScheduledFailuresDoNotCountWhileSubdomainModeIsOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		got := scheduledFailure(t, svc)
		if got.WildcardFailures != 0 || got.WildcardStatus != WildcardFailed || got.SubdomainDisabledReason != "" {
			t.Errorf("failure with the mode off = %+v", got)
		}
	})
}

func TestSuccessResetsTheCounterAndTheReasonButNotTheMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		for _, scheduled := range []bool{true, false} {
			svc := New(db, Options{})
			setModes(t, svc, true, true)
			scheduledFailure(t, svc)
			scheduledFailure(t, svc)
			scheduledFailure(t, svc)

			var ok Settings
			var err error
			if scheduled {
				ok, err = svc.RecordScheduledWildcard(ctx, true, "")
			} else {
				ok, err = svc.RecordWildcard(ctx, true, "")
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok.WildcardFailures != 0 || ok.SubdomainDisabledReason != "" || ok.WildcardStatus != WildcardOK || ok.SubdomainEnabled {
				t.Fatalf("scheduled=%t after success = %+v", scheduled, ok)
			}
			on, err := svc.Update(ctx, patch(t, `{"hosting.subdomainEnabled": true}`))
			if err != nil || !on.SubdomainEnabled {
				t.Fatalf("re-enabling = %+v, %v", on, err)
			}
			if _, err := svc.Update(ctx, patch(t, `{"hosting.subdomainEnabled": false}`)); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestEnablingSubdomainModeClearsTheDisableReason(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		setModes(t, svc, true, true)
		for range 3 {
			scheduledFailure(t, svc)
		}
		if _, err := svc.Update(ctx, patch(t, `{"hosting.subdomainEnabled": true}`)); !errors.Is(err, ErrWildcardNotVerified) {
			t.Fatalf("enabling after a failure: error = %v", err)
		}
		if _, err := svc.RecordWildcard(ctx, true, ""); err != nil {
			t.Fatal(err)
		}
		on, err := svc.Update(ctx, patch(t, `{"hosting.subdomainEnabled": true}`))
		if err != nil || on.SubdomainDisabledReason != "" || on.WildcardFailures != 0 {
			t.Errorf("after re-enabling = %+v, %v", on, err)
		}
	})
}

func TestMovingTheBaseDomainClearsTheFailureState(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		if _, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.com"}`)); err != nil {
			t.Fatal(err)
		}
		setModes(t, svc, true, true)
		for range 3 {
			scheduledFailure(t, svc)
		}
		moved, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.org"}`))
		if err != nil || moved.WildcardFailures != 0 || moved.SubdomainDisabledReason != "" || moved.WildcardStatus != WildcardUnchecked {
			t.Errorf("after moving the domain = %+v, %v", moved, err)
		}
	})
}

func TestFailureSettingsAreReadOnlyAndListed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		for _, raw := range []string{`{"hosting.wildcardFailures": 0}`, `{"hosting.subdomainDisabledReason": "x"}`} {
			if _, err := svc.Update(context.Background(), patch(t, raw)); !isValidation(err) {
				t.Errorf("%s: error = %v, want a validation error", raw, err)
			}
		}
		values := Values(Defaults())
		if values[KeyWildcardFailures] != int64(0) || values[KeyDisabledReason] != nil {
			t.Errorf("defaults = %v", values)
		}
	})
}
