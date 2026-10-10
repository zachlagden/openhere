package analytics

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/zachlagden/selfhost/internal/validate"
)

const (
	IntervalDay  = "day"
	IntervalHour = "hour"

	DefaultRangeDays = 30
	MaxRangeDays     = 366
	MaxHourlyDays    = 14
	DefaultTop       = 10
	MaxTop           = 50
)

type Params struct {
	From     time.Time
	To       time.Time
	Interval string
	Top      int
}

func (p Params) days() int {
	return int(p.To.Sub(p.From).Hours()/24) + 1
}

func ParseParams(query url.Values, now time.Time) (Params, error) {
	today := truncateDay(now)
	params := Params{To: today, Interval: IntervalDay, Top: DefaultTop}
	var err error
	if raw := query.Get("to"); raw != "" {
		if params.To, err = parseDate("to", raw); err != nil {
			return Params{}, err
		}
	}
	params.From = params.To.AddDate(0, 0, -(DefaultRangeDays - 1))
	if raw := query.Get("from"); raw != "" {
		if params.From, err = parseDate("from", raw); err != nil {
			return Params{}, err
		}
	}
	if params.From.After(params.To) {
		return Params{}, validate.Invalid("Parameter from must not be after to.")
	}
	if params.days() > MaxRangeDays {
		return Params{}, validate.Invalid(fmt.Sprintf("The range must not be longer than %d days.", MaxRangeDays))
	}
	if err := parseInterval(query.Get("interval"), &params); err != nil {
		return Params{}, err
	}
	return params, parseTop(query.Get("top"), &params)
}

func parseInterval(raw string, params *Params) error {
	switch raw {
	case "", IntervalDay:
		return nil
	case IntervalHour:
		if params.days() > MaxHourlyDays {
			return validate.Invalid(fmt.Sprintf("Hourly intervals need a range of %d days or less.", MaxHourlyDays))
		}
		params.Interval = IntervalHour
		return nil
	default:
		return validate.Invalid("Parameter interval must be day or hour.")
	}
}

func parseTop(raw string, params *Params) error {
	if raw == "" {
		return nil
	}
	top, err := strconv.Atoi(raw)
	if err != nil || top < 1 || top > MaxTop {
		return validate.Invalid(fmt.Sprintf("Parameter top must be between 1 and %d.", MaxTop))
	}
	params.Top = top
	return nil
}

func parseDate(name, raw string) (time.Time, error) {
	if day, err := time.Parse(time.DateOnly, raw); err == nil {
		return day, nil
	}
	stamp, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, validate.Invalid(fmt.Sprintf("Parameter %s must be an RFC 3339 date such as 2026-03-01.", name))
	}
	return truncateDay(stamp), nil
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
