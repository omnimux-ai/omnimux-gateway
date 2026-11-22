package analytics

import (
	"math"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func (c *client) payload(event Event) (umamiPayload, bool) {
	if event.UserID <= 0 {
		return umamiPayload{}, false
	}
	userKey := common.HmacSha256("user:"+strconv.Itoa(event.UserID), c.userKeySecret)
	data := make(map[string]any)
	switch event.Name {
	case EventSignUpSuccess:
		method, _ := event.Data["method"].(string)
		switch method {
		case "password", "oauth", "wechat":
		default:
			return umamiPayload{}, false
		}
		deviceID, _ := event.Data["device_id"].(string)
		appVersion, _ := event.Data["app_version"].(string)
		referrer, _ := event.Data["referrer"].(string)
		data = SignupData(method, deviceID, appVersion, referrer)
		// A syntactically valid hostname can still contain a tenant or account ID.
		normalizedReferrer, _ := data["referrer"].(string)
		if host, _, _ := strings.Cut(normalizedReferrer, "/"); host != c.hostname {
			data["referrer"] = ""
		}
	case EventFirstVideoStart, EventFirstVideoSuccess:
		taskID := identifier(event.Data["task_id"])
		if taskID == "" {
			return umamiPayload{}, false
		}
		data["task_id"] = taskID
		data["model_name"] = identifier(event.Data["model_name"])
		if event.Name == EventFirstVideoStart {
			entry, _ := event.Data["entry"].(string)
			switch entry {
			case "api", "workbench", "cli":
				data["entry"] = entry
			default:
				return umamiPayload{}, false
			}
			data["template_id"] = identifier(event.Data["template_id"])
		} else {
			data["user_key"] = userKey
			data["output_type"] = "video"
			data["is_first_success"] = true
			for _, key := range []string{"duration_ms", "file_size"} {
				if n, ok := event.Data[key].(int64); ok && n > 0 {
					data[key] = n
				}
			}
		}
	case EventTopUpPaid:
		amount, amountOK := event.Data["amount"].(float64)
		paidAt, timeOK := event.Data["paid_at"].(int64)
		if event.OrderRef == "" || !amountOK || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) ||
			!timeOK || paidAt <= 0 {
			return umamiPayload{}, false
		}
		data["order_id"] = common.HmacSha256("order:"+event.OrderRef, c.userKeySecret)
		data["amount"] = amount
		data["paid_at"] = paidAt
		data["currency"] = currencyCode(event.Data["currency"])
		data["payment_method"] = identifier(event.Data["payment_method"])
		data["pack_id"] = ""
	default:
		return umamiPayload{}, false
	}
	payload := umamiPayload{
		Website:  c.siteID,
		Hostname: c.hostname,
		URL:      "/server/" + event.Name,
		Name:     event.Name,
		ID:       userKey,
		Data:     data,
	}
	// Umami accepts integer Unix seconds through the 32-bit timestamp limit.
	if event.OccurredAt > 0 && event.OccurredAt <= math.MaxInt32 {
		payload.Timestamp = event.OccurredAt
	}
	return payload, true
}

var (
	signupDeviceIDPattern = regexp.MustCompile(`^[A-Fa-f0-9]{8}-[A-Fa-f0-9]{4}-[47][A-Fa-f0-9]{3}-[89ABab][A-Fa-f0-9]{3}-[A-Fa-f0-9]{12}$`)
	signupVersionPattern  = regexp.MustCompile(`^v?[0-9]{1,5}(?:\.[0-9]{1,5}){0,3}(?:-(?:alpha|beta|rc)(?:[.-]?[0-9]{1,5})?)?(?:\+(?:build\.)?[0-9]{1,6})?$`)
)

// SignupData keeps device UUIDs, numeric release versions and build numbers,
// and public referrers.
func SignupData(method, deviceID, appVersion, referrer string) map[string]any {
	if len(deviceID) > 64 || !signupDeviceIDPattern.MatchString(deviceID) {
		deviceID = ""
	}
	// Bare dotted numbers with seven or more digits can be phone numbers.
	versionMarked := strings.HasPrefix(appVersion, "v") || strings.ContainsAny(appVersion, "-+")
	phoneLike := !versionMarked && len(appVersion)-strings.Count(appVersion, ".") >= 7
	if len(appVersion) > 64 || !signupVersionPattern.MatchString(appVersion) || net.ParseIP(appVersion) != nil || phoneLike {
		appVersion = ""
	}
	return map[string]any{
		"method":      method,
		"device_id":   deviceID,
		"app_version": appVersion,
		"referrer":    referrerHostAndPath(referrer),
	}
}

func identifier(value any) string {
	s, ok := value.(string)
	if !ok || s == "" || len(s) > 128 || strings.HasPrefix(s, "/") ||
		strings.Contains(s, "://") || strings.Contains(s, "..") || net.ParseIP(s) != nil {
		return ""
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("-_./:", r) {
			continue
		}
		return ""
	}
	return s
}

func currencyCode(value any) string {
	s, _ := value.(string)
	s = strings.ToUpper(s)
	if len(s) != 3 {
		return ""
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}
	return s
}

// Keep only the publicly recognizable part of a referrer. Unknown path
// segments can carry tokens or account identifiers, even without a query.
func referrerHostAndPath(raw string) string {
	if raw == "" || len(raw) > 2048 {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw // Accept an already sanitized host and path.
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	host := canonicalHostname(u.Hostname())
	domainEnd := host[strings.LastIndexByte(host, '.')+1:]
	if !strings.Contains(host, ".") || domainEnd[0] < 'a' || domainEnd[0] > 'z' {
		return ""
	}
	switch u.EscapedPath() {
	case "/", "/login", "/register", "/sign-in", "/sign-up", "/pricing", "/models", "/influencer", "/ai-influencer-studio",
		"/tools/tiktok-photo-downloader", "/tools/tiktok-video-downloader",
		"/zh/tools/tiktok-photo-downloader", "/zh/tools/tiktok-video-downloader":
		return host + u.EscapedPath()
	default:
		return host
	}
}

func canonicalHostname(host string) string {
	if host == "" || len(host) > 100 {
		return ""
	}
	for _, r := range host {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return ""
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for label := range strings.SplitSeq(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return ""
		}
	}
	return host
}
