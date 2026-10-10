package settings

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestNoPatchLeavesBothHostingModesOff(t *testing.T) {
	cases := []struct {
		name            string
		subdomain, path bool
		patch           string
		refused         bool
	}{
		{"turn off path while subdomain is off", false, true, `{"hosting.pathEnabled": false}`, true},
		{"turn off subdomain while path is off", true, false, `{"hosting.subdomainEnabled": false}`, true},
		{"turn off both at once", true, true, `{"hosting.subdomainEnabled": false, "hosting.pathEnabled": false}`, true},
		{"swap subdomain for path", true, false, `{"hosting.subdomainEnabled": false, "hosting.pathEnabled": true}`, false},
		{"swap path for subdomain", false, true, `{"hosting.subdomainEnabled": true, "hosting.pathEnabled": false}`, false},
		{"turn off path while subdomain is on", true, true, `{"hosting.pathEnabled": false}`, false},
		{"turn off subdomain while path is on", true, true, `{"hosting.subdomainEnabled": false}`, false},
		{"unrelated change while one mode is on", false, true, `{"retention.keepVersions": 4}`, false},
		{"turn the other mode on", false, true, `{"hosting.subdomainEnabled": true}`, false},
	}
	for _, tc := range cases {
		storetest.Each(t, func(t *testing.T, db store.Store) {
			ctx := context.Background()
			svc := New(db, Options{})
			setModes(t, svc, tc.subdomain, tc.path)
			before, err := svc.Get(ctx)
			if err != nil {
				t.Fatal(err)
			}
			after, err := svc.Update(ctx, patch(t, tc.patch))
			if tc.refused {
				if !errors.Is(err, ErrNoHostingMode) {
					t.Fatalf("%s: error = %v, want ErrNoHostingMode", tc.name, err)
				}
				stored, getErr := svc.Get(ctx)
				if getErr != nil || stored != before {
					t.Errorf("%s: a refused change altered the settings: %+v, %v", tc.name, stored, getErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !after.SubdomainEnabled && !after.PathEnabled {
				t.Errorf("%s: both modes ended up off: %+v", tc.name, after)
			}
		})
	}
}

func setModes(t *testing.T, svc *Service, subdomain, path bool) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.RecordWildcard(ctx, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, patch(t, fmt.Sprintf(`{"hosting.subdomainEnabled": %t, "hosting.pathEnabled": %t}`, subdomain, path))); err != nil {
		t.Fatal(err)
	}
}

func TestChangingTheBaseDomainKeepsAModeOn(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		ctx := context.Background()
		svc := New(db, Options{})
		if _, err := svc.Update(ctx, patch(t, `{"hosting.baseDomain": "example.com"}`)); err != nil {
			t.Fatal(err)
		}
		setModes(t, svc, true, false)

		outcome, err := svc.Apply(ctx, patch(t, `{"hosting.baseDomain": "example.org"}`))
		if err != nil {
			t.Fatal(err)
		}
		got := outcome.Settings
		if got.SubdomainEnabled || !got.PathEnabled || got.WildcardStatus != WildcardUnchecked {
			t.Errorf("after moving the domain = %+v", got)
		}
		if len(outcome.Notices) != 1 || outcome.Notices[0].Code != NoticePathModeEnabled || outcome.Notices[0].Message == "" {
			t.Errorf("notices = %+v", outcome.Notices)
		}

		again, err := svc.Apply(ctx, patch(t, `{"hosting.baseDomain": "example.net"}`))
		if err != nil || len(again.Notices) != 0 {
			t.Errorf("moving the domain with path mode on: notices %+v, error %v", again.Notices, err)
		}
	})
}

func TestChangingTheBaseDomainCannotBeCombinedWithSwitchingEveryModeOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := New(db, Options{})
		setModes(t, svc, true, true)
		_, err := svc.Update(context.Background(), patch(t, `{"hosting.baseDomain": "example.org", "hosting.subdomainEnabled": false, "hosting.pathEnabled": false}`))
		if !errors.Is(err, ErrNoHostingMode) {
			t.Errorf("error = %v, want ErrNoHostingMode", err)
		}
	})
}
