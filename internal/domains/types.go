package domains

import (
	"errors"
	"time"
)

const (
	StatusPending  = "pending_verification"
	StatusVerified = "verified"

	ResultVerified     = "verified"
	ResultNotFound     = "record_not_found"
	ResultMismatch     = "token_mismatch"
	ResultLookupFailed = "lookup_failed"

	MissingChecksToDrop = 3
	StaleClaimAge       = 7 * 24 * time.Hour
	RecheckInterval     = 24 * time.Hour
)

var (
	ErrNotFound     = errors.New("domain not found")
	ErrTaken        = errors.New("domain is already in use")
	ErrLimitReached = errors.New("site has reached its custom domain limit")
	ErrNotVerified  = errors.New("domain is not verified")
)

type Domain struct {
	ID              string
	SiteID          string
	Name            string
	Token           string
	VerifiedAt      *time.Time
	LastCheckedAt   *time.Time
	LastCheckResult string
	MissingChecks   int
	IsPrimary       bool
	CreatedAt       time.Time
}

func (d Domain) Verified() bool {
	return d.VerifiedAt != nil
}

func (d Domain) Status() string {
	if d.Verified() {
		return StatusVerified
	}
	return StatusPending
}

func (d Domain) ChallengeName() string {
	return ChallengeLabel + "." + d.Name
}

func (d Domain) ResultMessage() string {
	switch d.LastCheckResult {
	case ResultVerified:
		return "The TXT record was found and matches."
	case ResultNotFound:
		return "No TXT record was found at " + d.ChallengeName() + ". DNS changes can take a while to spread, so try again later."
	case ResultMismatch:
		return "TXT records exist at " + d.ChallengeName() + ", but none holds the value shown. Check that you copied it exactly."
	case ResultLookupFailed:
		return "The DNS lookup failed or timed out. This is usually temporary, so try again in a few minutes."
	default:
		return ""
	}
}

type Summary struct {
	Total    int
	Verified int
	Primary  string
	Fallback string
}

func (s Summary) Address() string {
	if s.Primary != "" {
		return s.Primary
	}
	return s.Fallback
}

type Target struct {
	SiteID string
	Slug   string
	Domain string
}
