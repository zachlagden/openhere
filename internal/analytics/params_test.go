package analytics

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/sites"
	"github.com/zachlagden/selfhost/internal/validate"
)

func siteInput(userID, workspace, slug string) sites.CreateInput {
	return sites.CreateInput{UserID: userID, Workspace: workspace, Slug: slug}
}

func TestParseParams(t *testing.T) {
	now := time.Date(2026, 3, 10, 22, 30, 0, 0, time.UTC)
	good := []struct {
		name  string
		query string
		want  Params
	}{
		{"defaults", "", Params{From: day("2026-02-09"), To: day("2026-03-10"), Interval: IntervalDay, Top: 10}},
		{"dates", "from=2026-03-01&to=2026-03-05&top=3", Params{From: day("2026-03-01"), To: day("2026-03-05"), Interval: IntervalDay, Top: 3}},
		{"timestamps", "from=2026-03-01T23:59:00Z&to=2026-03-05T00:00:00Z", Params{From: day("2026-03-01"), To: day("2026-03-05"), Interval: IntervalDay, Top: 10}},
		{"offset timestamps use utc", "from=2026-03-02T01:00:00%2B02:00&to=2026-03-02T01:00:00%2B02:00", Params{From: day("2026-03-01"), To: day("2026-03-01"), Interval: IntervalDay, Top: 10}},
		{"hourly", "from=2026-03-01&to=2026-03-14&interval=hour", Params{From: day("2026-03-01"), To: day("2026-03-14"), Interval: IntervalHour, Top: 10}},
		{"only to", "to=2026-03-31", Params{From: day("2026-03-02"), To: day("2026-03-31"), Interval: IntervalDay, Top: 10}},
	}
	for _, c := range good {
		t.Run(c.name, func(t *testing.T) {
			query, err := url.ParseQuery(c.query)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseParams(query, now)
			if err != nil || got != c.want {
				t.Errorf("ParseParams = %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
	bad := []string{
		"from=yesterday", "to=2026-13-01", "from=2026-03-05&to=2026-03-01", "from=2025-01-01&to=2026-03-01",
		"interval=week", "from=2026-03-01&to=2026-03-20&interval=hour", "top=0", "top=51", "top=x",
	}
	for _, raw := range bad {
		t.Run("bad "+raw, func(t *testing.T) {
			query, err := url.ParseQuery(raw)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseParams(query, now)
			var invalid *validate.Error
			if !errors.As(err, &invalid) {
				t.Errorf("ParseParams(%q) error = %v, want a validation error", raw, err)
			}
		})
	}
}
