package domains

import (
	"context"
	"errors"
	"testing"
)

func TestVerifyAcceptsTheChallengeToken(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.claim(siteID, "www.example.org")
		e.publishChallenge(d)
		got, err := e.svc.Verify(context.Background(), siteID, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Verified() || got.Status() != StatusVerified || got.LastCheckResult != ResultVerified || got.LastCheckedAt == nil {
			t.Errorf("domain = %+v", got)
		}
		if !e.serves("www.example.org") {
			t.Error("a verified domain does not serve")
		}
	})
}

func TestVerifyOutcomes(t *testing.T) {
	long := repeatString("x", maxRecordLength+1)
	many := make([]string, maxRecordsRead)
	for i := range many {
		many[i] = "noise"
	}
	cases := []struct {
		name    string
		records func(token string) []string
		err     error
		missing bool
		result  string
		ok      bool
	}{
		{"matching record", func(t string) []string { return []string{t} }, nil, false, ResultVerified, true},
		{"quoted record", func(t string) []string { return []string{`"` + t + `"`} }, nil, false, ResultVerified, true},
		{"padded record", func(t string) []string { return []string{"  " + t + " "} }, nil, false, ResultVerified, true},
		{"one of many records", func(t string) []string { return []string{"v=spf1 -all", "other", t, "more"} }, nil, false, ResultVerified, true},
		{"wrong token", func(string) []string { return []string{"not-the-token"} }, nil, false, ResultMismatch, false},
		{"token as a substring", func(t string) []string { return []string{"prefix-" + t} }, nil, false, ResultMismatch, false},
		{"token with a suffix", func(t string) []string { return []string{t + "x"} }, nil, false, ResultMismatch, false},
		{"upper case token", func(t string) []string { return []string{upper(t)} }, nil, false, ResultMismatch, false},
		{"oversized record holding the token", func(t string) []string { return []string{t + long} }, nil, false, ResultMismatch, false},
		{"token after too many records", func(t string) []string { return append(append([]string{}, many...), t) }, nil, false, ResultMismatch, false},
		{"empty answer", func(string) []string { return nil }, nil, false, ResultNotFound, false},
		{"no record at all", nil, nil, true, ResultNotFound, false},
		{"authoritative no such name", func(string) []string { return nil }, &netNotFound, false, ResultNotFound, false},
		{"temporary failure", func(string) []string { return nil }, transient(), false, ResultLookupFailed, false},
		{"timeout", func(string) []string { return nil }, timeout(), false, ResultLookupFailed, false},
		{"plain error", func(string) []string { return nil }, errors.New("connection refused"), false, ResultLookupFailed, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			each(t, func(t *testing.T, e *env) {
				siteID := e.site("launch")
				d := e.claim(siteID, "www.example.org")
				if !tc.missing {
					e.dns.set(d.ChallengeName(), tc.records(d.Token), tc.err)
				}
				got, err := e.svc.Verify(context.Background(), siteID, d.ID)
				if err != nil {
					t.Fatalf("%s: %v", tc.name, err)
				}
				if got.Verified() != tc.ok || got.LastCheckResult != tc.result {
					t.Errorf("%s: verified = %v, result = %q, want %v and %q", tc.name, got.Verified(), got.LastCheckResult, tc.ok, tc.result)
				}
				if got.ResultMessage() == "" {
					t.Errorf("%s: no message for result %q", tc.name, got.LastCheckResult)
				}
				if got.LastCheckedAt == nil {
					t.Errorf("%s: the check time was not recorded", tc.name)
				}
				if e.serves("www.example.org") != tc.ok {
					t.Errorf("%s: serves = %v, want %v", tc.name, !tc.ok, tc.ok)
				}
			})
		})
	}
}

func TestVerifyDoesNotConsultTheDomainItself(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.claim(siteID, "www.example.org")
		e.dns.set("www.example.org", []string{d.Token}, nil)
		got, err := e.svc.Verify(context.Background(), siteID, d.ID)
		if err != nil || got.Verified() {
			t.Errorf("a token at the domain itself verified it: %+v, %v", got, err)
		}
		for _, query := range e.dns.queries {
			if query != d.ChallengeName() {
				t.Errorf("queried %s, want only %s", query, d.ChallengeName())
			}
		}
	})
}

func TestVerifyScopesToTheSite(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		mine, theirs := e.site("launch"), e.site("guides")
		d := e.claim(theirs, "www.example.org")
		e.publishChallenge(d)
		if _, err := e.svc.Verify(context.Background(), mine, d.ID); !isErr(err, ErrNotFound) {
			t.Errorf("Verify through the wrong site = %v, want ErrNotFound", err)
		}
		if _, err := e.svc.Verify(context.Background(), theirs, "missing"); !isErr(err, ErrNotFound) {
			t.Errorf("Verify of an unknown id = %v, want ErrNotFound", err)
		}
		if e.get(theirs, d.ID).Verified() {
			t.Error("a rejected verify changed the domain")
		}
	})
}

func TestManualVerifyNeverUnverifiesADomain(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		e.dns.clear(d.ChallengeName())
		for range MissingChecksToDrop + 2 {
			got, err := e.svc.Verify(context.Background(), siteID, d.ID)
			if err != nil || !got.Verified() || got.LastCheckResult != ResultNotFound {
				t.Fatalf("Verify = %+v, %v", got, err)
			}
		}
		if got := e.get(siteID, d.ID); got.MissingChecks != 0 {
			t.Errorf("manual checks counted %d misses", got.MissingChecks)
		}
	})
}

func TestVerifyAllowsOnlyOneClaimantToTakeADomain(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		first, second := e.site("launch"), e.site("guides")
		a := e.claim(first, "www.example.org")
		e.publishChallenge(a)
		if _, err := e.svc.Claim(context.Background(), second, "www.example.org"); err != nil {
			t.Fatalf("second claim = %v", err)
		}
		if _, err := e.svc.Verify(context.Background(), first, a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Claim(context.Background(), second, "www.example.org"); !isErr(err, ErrTaken) {
			t.Fatalf("claim after verification = %v", err)
		}
		target, ok, err := e.svc.Lookup(context.Background(), "www.example.org")
		if err != nil || !ok || target.SiteID != first || target.Slug != "launch" {
			t.Errorf("Lookup = %+v, %v, %v", target, ok, err)
		}
	})
}

var netNotFound = notFoundError()
