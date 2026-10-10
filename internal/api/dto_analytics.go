package api

import (
	"time"

	"github.com/zachlagden/selfhost/internal/analytics"
)

type analyticsTotalsResponse struct {
	Views    int64 `json:"views"`
	Uniques  int64 `json:"uniques"`
	BotViews int64 `json:"botViews"`
}

type analyticsPointResponse struct {
	Start   time.Time `json:"start"`
	Views   int64     `json:"views"`
	Uniques int64     `json:"uniques"`
}

type analyticsRankedResponse struct {
	Value   string `json:"value"`
	Views   int64  `json:"views"`
	Uniques int64  `json:"uniques"`
}

type analyticsResponse struct {
	From              string                    `json:"from"`
	To                string                    `json:"to"`
	Interval          string                    `json:"interval"`
	Totals            analyticsTotalsResponse   `json:"totals"`
	Series            []analyticsPointResponse  `json:"series"`
	Paths             []analyticsRankedResponse `json:"paths"`
	Referrers         []analyticsRankedResponse `json:"referrers"`
	Countries         []analyticsRankedResponse `json:"countries"`
	Cities            []analyticsRankedResponse `json:"cities"`
	Browsers          []analyticsRankedResponse `json:"browsers"`
	OperatingSystems  []analyticsRankedResponse `json:"operatingSystems"`
	Devices           []analyticsRankedResponse `json:"devices"`
	Statuses          []analyticsRankedResponse `json:"statuses"`
	LocationAvailable bool                      `json:"locationAvailable"`
	DroppedEvents     *int64                    `json:"droppedEvents,omitempty"`
	GeneratedAt       time.Time                 `json:"generatedAt"`
}

func toAnalyticsResponse(r analytics.Report) analyticsResponse {
	series := make([]analyticsPointResponse, len(r.Series))
	for i, p := range r.Series {
		series[i] = analyticsPointResponse{Start: p.Start, Views: p.Views, Uniques: p.Uniques}
	}
	return analyticsResponse{
		From: r.From.Format(analytics.DayLayout), To: r.To.Format(analytics.DayLayout), Interval: r.Interval,
		Totals: analyticsTotalsResponse{Views: r.Totals.Views, Uniques: r.Totals.Uniques, BotViews: r.Totals.BotViews},
		Series: series, Paths: toRankedResponse(r.Paths), Referrers: toRankedResponse(r.Referrers),
		Countries: toRankedResponse(r.Countries), Cities: toRankedResponse(r.Cities), Browsers: toRankedResponse(r.Browsers),
		OperatingSystems: toRankedResponse(r.Systems), Devices: toRankedResponse(r.Devices), Statuses: toRankedResponse(r.Statuses),
		LocationAvailable: r.LocationAvailable, DroppedEvents: r.DroppedEvents, GeneratedAt: r.GeneratedAt,
	}
}

func toRankedResponse(list []analytics.Ranked) []analyticsRankedResponse {
	out := make([]analyticsRankedResponse, len(list))
	for i, item := range list {
		out[i] = analyticsRankedResponse{Value: item.Value, Views: item.Views, Uniques: item.Uniques}
	}
	return out
}
