package analytics

import (
	"strings"
	"testing"
)

func TestParseUserAgent(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want Agent
	}{
		{"chrome on windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", Agent{"Chrome", "Windows", DeviceDesktop}},
		{"edge on windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0", Agent{"Edge", "Windows", DeviceDesktop}},
		{"firefox on linux", "Mozilla/5.0 (X11; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0", Agent{"Firefox", "Linux", DeviceDesktop}},
		{"safari on macos", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", Agent{"Safari", "macOS", DeviceDesktop}},
		{"safari on iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", Agent{"Safari", "iOS", DeviceMobile}},
		{"chrome on ios", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.0.0 Mobile/15E148 Safari/604.1", Agent{"Chrome", "iOS", DeviceMobile}},
		{"safari on ipad", "Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", Agent{"Safari", "iOS", DeviceTablet}},
		{"chrome on android phone", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36", Agent{"Chrome", "Android", DeviceMobile}},
		{"chrome on android tablet", "Mozilla/5.0 (Linux; Android 14; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", Agent{"Chrome", "Android", DeviceTablet}},
		{"samsung internet", "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36", Agent{"Samsung Internet", "Android", DeviceMobile}},
		{"opera", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 OPR/111.0.0.0", Agent{"Opera", "Windows", DeviceDesktop}},
		{"internet explorer", "Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko", Agent{"Internet Explorer", "Windows", DeviceDesktop}},
		{"chromeos", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", Agent{"Chrome", "ChromeOS", DeviceDesktop}},
		{"googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", Agent{"Other", "Other", DeviceBot}},
		{"curl", "curl/8.5.0", Agent{"Other", "Other", DeviceBot}},
		{"go client", "Go-http-client/2.0", Agent{"Other", "Other", DeviceBot}},
		{"headless chrome", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/126.0.0.0 Safari/537.36", Agent{"Chrome", "Linux", DeviceBot}},
		{"empty", "", Agent{"Other", "Other", DeviceBot}},
		{"unknown", "SomethingCustom/1.0", Agent{"Other", "Other", DeviceOther}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseUserAgent(c.ua); got != c.want {
				t.Errorf("ParseUserAgent = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestReferrerHost(t *testing.T) {
	cases := []struct{ name, referrer, own, want string }{
		{"plain", "https://news.example.org/story?id=7#top", "demo.example.com", "news.example.org"},
		{"www is stripped", "https://www.Example.org/", "demo.example.com", "example.org"},
		{"port is dropped", "http://search.example.net:8080/q", "demo.example.com", "search.example.net"},
		{"empty", "", "demo.example.com", ""},
		{"not a url", "somewhere", "demo.example.com", ""},
		{"other scheme", "android-app://com.example.app", "demo.example.com", ""},
		{"own host", "https://demo.example.com/about", "demo.example.com", ""},
		{"own host with port", "https://demo.example.com/about", "demo.example.com:8080", ""},
		{"userinfo is dropped", "https://user:pass@docs.example.org/", "demo.example.com", "docs.example.org"},
		{"garbage", "https://%zz/", "demo.example.com", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ReferrerHost(c.referrer, c.own); got != c.want {
				t.Errorf("ReferrerHost = %q, want %q", got, c.want)
			}
		})
	}
}

func TestNormalisePath(t *testing.T) {
	long := "/" + strings.Repeat("a", 300)
	cases := []struct{ name, in, want string }{
		{"plain", "/docs/intro", "/docs/intro"},
		{"query is cut", "/docs?token=abc", "/docs"},
		{"fragment is cut", "/docs#section", "/docs"},
		{"trailing slash stays", "/docs/", "/docs/"},
		{"root", "/", "/"},
		{"missing slash", "docs", "/docs"},
		{"controls are removed", "/a\x00b\nc", "/abc"},
		{"invalid utf8 is removed", "/a\xffb", "/ab"},
		{"unicode stays", "/café", "/café"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalisePath(c.in); got != c.want {
				t.Errorf("NormalisePath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
	if got := NormalisePath(long); len(got) != MaxPathLength {
		t.Errorf("long path length = %d, want %d", len(got), MaxPathLength)
	}
}
