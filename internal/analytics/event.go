package analytics

import (
	"net/netip"
	"time"
)

const (
	StatusSuccess  = "2xx"
	StatusNotFound = "4xx"
)

type Hit struct {
	SiteID    string
	At        time.Time
	Path      string
	Status    int
	Referrer  string
	Host      string
	Addr      netip.Addr
	UserAgent string
}

type Event struct {
	SiteID       string
	OccurredAt   time.Time
	Path         string
	StatusClass  string
	ReferrerHost string
	Country      string
	City         string
	Browser      string
	OS           string
	Device       string
	Visitor      string
}

func (e Event) Day() string {
	return e.OccurredAt.UTC().Format(DayLayout)
}

func (e Event) Hour() int {
	return e.OccurredAt.UTC().Hour()
}

func statusClass(status int) (string, bool) {
	switch {
	case status >= 200 && status < 300:
		return StatusSuccess, true
	case status == 404:
		return StatusNotFound, true
	default:
		return "", false
	}
}
