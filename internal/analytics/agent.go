package analytics

import "strings"

const (
	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceBot     = "bot"
	DeviceOther   = "other"

	unknownName = "Other"
)

type Agent struct {
	Browser string
	OS      string
	Device  string
}

type marker struct {
	token string
	name  string
}

var botTokens = []string{
	"bot", "crawl", "spider", "slurp", "scrape", "fetch", "monitor", "uptime", "probe", "headless", "lighthouse",
	"preview", "curl/", "wget/", "python-requests", "python-urllib", "aiohttp", "go-http-client", "okhttp", "java/",
	"libwww", "httpclient", "http-client", "postman", "insomnia", "axios", "node-fetch", "undici", "scrapy",
	"facebookexternalhit", "whatsapp/", "slack-", "skype", "pingdom", "datadog", "newrelic",
	"gtmetrix", "chrome-lighthouse", "pagespeed", "phantomjs", "ahrefs", "semrush", "mj12", "dotbot",
	"bing", "google-", "apis-google", "mediapartners", "feedly",
}

var browserMarkers = []marker{
	{"edgios", "Edge"}, {"edga", "Edge"}, {"edg/", "Edge"}, {"edge/", "Edge"},
	{"opr/", "Opera"}, {"opera", "Opera"}, {"opios", "Opera"},
	{"samsungbrowser", "Samsung Internet"},
	{"vivaldi", "Vivaldi"},
	{"firefox/", "Firefox"}, {"fxios", "Firefox"},
	{"crios", "Chrome"}, {"chromium", "Chromium"}, {"chrome/", "Chrome"},
	{"msie", "Internet Explorer"}, {"trident/", "Internet Explorer"},
}

var osMarkers = []marker{
	{"windows phone", "Windows Phone"},
	{"windows", "Windows"},
	{"iphone", "iOS"}, {"ipad", "iOS"}, {"ipod", "iOS"},
	{"android", "Android"},
	{"cros", "ChromeOS"},
	{"mac os x", "macOS"}, {"macintosh", "macOS"},
	{"linux", "Linux"}, {"x11", "Linux"},
}

func ParseUserAgent(raw string) Agent {
	ua := strings.ToLower(strings.TrimSpace(raw))
	return Agent{Browser: parseBrowser(ua), OS: firstMarker(ua, osMarkers), Device: parseDevice(ua)}
}

func parseBrowser(ua string) string {
	if name := firstMarker(ua, browserMarkers); name != unknownName {
		return name
	}
	if strings.Contains(ua, "safari/") && strings.Contains(ua, "version/") {
		return "Safari"
	}
	if strings.Contains(ua, "applewebkit") && strings.Contains(ua, "mobile/") {
		return "Safari"
	}
	return unknownName
}

func firstMarker(ua string, markers []marker) string {
	for _, m := range markers {
		if strings.Contains(ua, m.token) {
			return m.name
		}
	}
	return unknownName
}

func parseDevice(ua string) string {
	if ua == "" || isBot(ua) {
		return DeviceBot
	}
	switch {
	case isTablet(ua):
		return DeviceTablet
	case isMobile(ua):
		return DeviceMobile
	case isDesktop(ua):
		return DeviceDesktop
	default:
		return DeviceOther
	}
}

func isBot(ua string) bool {
	for _, token := range botTokens {
		if strings.Contains(ua, token) {
			return true
		}
	}
	return false
}

func isTablet(ua string) bool {
	if strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet") || strings.Contains(ua, "kindle") || strings.Contains(ua, "silk/") {
		return true
	}
	return strings.Contains(ua, "android") && !strings.Contains(ua, "mobile")
}

func isMobile(ua string) bool {
	for _, token := range []string{"iphone", "ipod", "mobile", "windows phone", "blackberry", "opera mini"} {
		if strings.Contains(ua, token) {
			return true
		}
	}
	return false
}

func isDesktop(ua string) bool {
	for _, token := range []string{"windows nt", "macintosh", "x11", "cros", "linux"} {
		if strings.Contains(ua, token) {
			return true
		}
	}
	return false
}
