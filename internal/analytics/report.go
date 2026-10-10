package analytics

import "time"

type Totals struct {
	Views    int64
	Uniques  int64
	BotViews int64
}

type Point struct {
	Start   time.Time
	Views   int64
	Uniques int64
}

type Ranked struct {
	Value   string
	Views   int64
	Uniques int64
}

type Report struct {
	From      time.Time
	To        time.Time
	Interval  string
	Totals    Totals
	Series    []Point
	Paths     []Ranked
	Referrers []Ranked
	Countries []Ranked
	Cities    []Ranked
	Browsers  []Ranked
	Systems   []Ranked
	Devices   []Ranked
	Statuses  []Ranked

	LocationAvailable bool
	DroppedEvents     *int64
	GeneratedAt       time.Time
}
