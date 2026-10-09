package hosting

import (
	"encoding/json"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/proxy"
	"github.com/zachlagden/selfhost/internal/ratelimit"
)

const (
	maxFormBytes   = 4 << 10
	maxLimiterKey  = 254
	rateLimitCode  = "rate_limited"
	rateLimitReply = "Too many attempts. Try again later."
)

func (g *gate) clientKey(r *http.Request) string {
	return proxy.LimiterKey(g.proxies.ClientIP(r))
}

func (g *gate) readForm(req siteRequest) (url.Values, bool) {
	mediaType, _, err := mime.ParseMediaType(req.r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		writeText(req.w, http.StatusUnsupportedMediaType, "Unsupported media type\n", req.r.Method != http.MethodHead)
		return nil, false
	}
	req.r.Body = http.MaxBytesReader(req.w, req.r.Body, maxFormBytes)
	if err := req.r.ParseForm(); err != nil {
		writeText(req.w, http.StatusBadRequest, "Bad request\n", true)
		return nil, false
	}
	return req.r.PostForm, true
}

func (g *gate) checkForm(req siteRequest, state access.State, values url.Values) bool {
	if !sameOrigin(req.r) {
		g.expired(req, state, values)
		return false
	}
	ok, err := g.access.VerifyFormToken(req.r.Context(), req.site.ID, state.Epoch, g.clientKey(req.r), values.Get("token"))
	if err != nil {
		g.fail(req.w, err)
		return false
	}
	if !ok {
		g.expired(req, state, values)
	}
	return ok
}

func (g *gate) expired(req siteRequest, state access.State, values url.Values) {
	g.renderPage(req, state, pageData{
		Status: http.StatusForbidden, Message: "This form has expired. Try again.", Return: values.Get("return"),
	})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func (g *gate) within(req siteRequest, state access.State, limiter *ratelimit.Limiter, key string) bool {
	ok, wait := limiter.Allow(key)
	if ok {
		return true
	}
	seconds := int(math.Ceil(wait.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	req.w.Header().Set("Retry-After", strconv.Itoa(seconds))
	if wantsJSON(req.r) {
		writeRetryJSON(req.w, seconds)
		return false
	}
	g.renderPage(req, state, pageData{
		Status: http.StatusTooManyRequests, Message: "Too many attempts. Try again in " + waitText(seconds) + ".",
	})
	return false
}

func waitText(seconds int) string {
	wait := time.Duration(seconds) * time.Second
	if wait >= 2*time.Minute {
		return strconv.Itoa(int(math.Ceil(wait.Minutes()))) + " minutes"
	}
	return strconv.Itoa(seconds) + " seconds"
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

func writeRetryJSON(w http.ResponseWriter, seconds int) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", cacheGatePage)
	w.WriteHeader(http.StatusTooManyRequests)
	body := map[string]any{"code": rateLimitCode, "message": rateLimitReply, "retry_after": seconds}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logWriteFailure(err)
	}
}

func returnPathFrom(raw string) string {
	if safe, ok := access.SafeReturnPath(raw); ok {
		return safe
	}
	return "/"
}

func emailKey(siteID, email string) string {
	key := strings.ToLower(strings.TrimSpace(email))
	if len(key) > maxLimiterKey {
		key = key[:maxLimiterKey]
	}
	return siteID + "|" + key
}
