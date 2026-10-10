package analytics

import (
	"context"
	"time"
	"unicode/utf8"
)

func (r *Recorder) build(hit Hit) (Event, bool) {
	class, counted := statusClass(hit.Status)
	if !counted {
		return Event{}, false
	}
	agent := ParseUserAgent(hit.UserAgent)
	event := Event{
		SiteID: hit.SiteID, OccurredAt: hit.At.UTC().Truncate(time.Second), Path: NormalisePath(hit.Path), StatusClass: class,
		ReferrerHost: ReferrerHost(hit.Referrer, hit.Host), Browser: agent.Browser, OS: agent.OS, Device: agent.Device,
	}
	if agent.Device == DeviceBot {
		return event, true
	}
	if r.locator != nil && hit.Addr.IsValid() {
		country, city := r.locator.Locate(hit.Addr)
		event.Country, event.City = cleanCountry(country), cleanCity(city)
	}
	ctx, cancel := context.WithTimeout(context.Background(), insertTimeout)
	defer cancel()
	visitor, err := r.hasher.Visitor(ctx, hit.Addr, hit.UserAgent, hit.SiteID, event.OccurredAt)
	if err != nil {
		r.logger.Warn("hash an analytics visitor", "error", err)
		return Event{}, false
	}
	event.Visitor = visitor
	return event, true
}

func cleanCountry(code string) string {
	if len(code) != 2 {
		return ""
	}
	return code
}

func cleanCity(name string) string {
	if utf8.RuneCountInString(name) <= maxCityLength {
		return name
	}
	return string([]rune(name)[:maxCityLength])
}
