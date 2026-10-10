package analytics

const (
	dimTotal    = "total"
	dimPath     = "path"
	dimReferrer = "referrer"
	dimCountry  = "country"
	dimCity     = "city"
	dimBrowser  = "browser"
	dimOS       = "os"
	dimDevice   = "device"
	dimStatus   = "status"
	dimEvents   = "events"

	humanViews = "status_class = '2xx' AND device <> 'bot'"
	allViews   = "status_class = '2xx'"
	humanAll   = "device <> 'bot'"

	maxRolledValues = 500
)

type dimension struct {
	name      string
	column    string
	where     string
	skipEmpty bool
}

var dimensions = []dimension{
	{name: dimTotal, where: humanViews},
	{name: dimPath, column: "path", where: humanViews},
	{name: dimReferrer, column: "referrer_host", where: humanViews, skipEmpty: true},
	{name: dimCountry, column: "country", where: humanViews, skipEmpty: true},
	{name: dimCity, column: "city", where: humanViews, skipEmpty: true},
	{name: dimBrowser, column: "browser", where: humanViews},
	{name: dimOS, column: "os", where: humanViews},
	{name: dimDevice, column: "device", where: allViews},
	{name: dimStatus, column: "status_class", where: humanAll},
}

func (d dimension) condition() string {
	if d.skipEmpty {
		return d.where + " AND " + d.column + " <> ''"
	}
	return d.where
}
