package controller

import (
	"archive/zip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/tidwall/gjson"
)

// fork: public TikTok video/photo tool pages. Anonymous visitors parse a
// public TikTok link through the existing tiktok-video capability on a
// dedicated channel. The browser never sees the upstream credential or the
// signed TikTok CDN URLs; media is fetched through short-lived signed paths.
//
// Abuse gates (all server-side, all fail closed):
//  1. same-origin request + TikTok host allowlist (short links resolved only
//     to TikTok hosts);
//  2. per-IP burst limit and per-day quota (per-user when signed in);
//  3. Cloudflare Turnstile (managed) on the first parse, then a signed,
//     httpOnly 2-hour pass bound to the client /24 (/48) and user agent;
//  4. 30-minute per-post result cache (cache hits never reach upstream);
//  5. global daily anonymous budget;
//  6. signed media links (10 minutes, IP bound, size capped, 1 concurrent
//     download per IP);
//  7. ZIP of photo posts requires a session.

const (
	tiktokToolPassCookie  = "omx_tt_pass"
	tiktokToolPassTTL     = 2 * time.Hour
	tiktokToolMediaTTL    = 10 * time.Minute
	tiktokToolCacheTTL    = 30 * time.Minute
	tiktokToolMaxImages   = 50
	tiktokToolMaxVideo    = 200 << 20
	tiktokToolMaxImage    = 30 << 20
	tiktokToolUpstreamTTL = 20 * time.Second
	tiktokToolModel       = "tiktok-video"
)

type tiktokToolSettings struct {
	enabled         bool
	channelID       int
	siteKey         string
	secretKey       string
	anonDaily       int
	userDaily       int
	globalAnonDaily int
	burstSeconds    int64
	allowedHosts    []string
}

// tiktokToolConfig reads deployment configuration. The Turnstile key pair is
// independent of the login/registration Turnstile option on purpose.
func tiktokToolConfig() tiktokToolSettings {
	return tiktokToolSettings{
		enabled:         common.GetEnvOrDefaultBool("TIKTOK_TOOL_ENABLED", true),
		channelID:       common.GetEnvOrDefault("TIKTOK_TOOL_CHANNEL_ID", 0),
		siteKey:         strings.TrimSpace(common.GetEnvOrDefaultString("TIKTOK_TOOL_TURNSTILE_SITE_KEY", "")),
		secretKey:       strings.TrimSpace(common.GetEnvOrDefaultString("TIKTOK_TOOL_TURNSTILE_SECRET_KEY", "")),
		anonDaily:       max(1, common.GetEnvOrDefault("TIKTOK_TOOL_ANON_DAILY", 5)),
		userDaily:       max(1, common.GetEnvOrDefault("TIKTOK_TOOL_USER_DAILY", 50)),
		globalAnonDaily: max(1, common.GetEnvOrDefault("TIKTOK_TOOL_GLOBAL_ANON_DAILY", 3000)),
		burstSeconds:    int64(max(1, common.GetEnvOrDefault("TIKTOK_TOOL_BURST_SECONDS", 10))),
		allowedHosts:    tiktokToolAllowedHosts(),
	}
}

// tiktokToolAllowedHosts are the page hosts accepted as Origin and as the
// Turnstile siteverify hostname. TIKTOK_TOOL_ALLOWED_HOSTS replaces the
// production pair (for staging or a local run with Cloudflare test keys).
func tiktokToolAllowedHosts() []string {
	raw := strings.TrimSpace(common.GetEnvOrDefaultString("TIKTOK_TOOL_ALLOWED_HOSTS", ""))
	if raw == "" {
		return []string{"omnimux.ai", "www.omnimux.ai"}
	}
	hosts := []string{}
	for host := range strings.SplitSeq(raw, ",") {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// ---------------------------------------------------------------- links

var tiktokLinkPattern = regexp.MustCompile(`https?://[A-Za-z0-9.-]*tiktok\.com/[^\s"'<>]+`)
var tiktokPostPathPattern = regexp.MustCompile(`^/@[^/]+/(video|photo)/(\d{8,24})/?$`)

type tiktokLink struct {
	raw  string
	id   string
	kind string // video | photo | "" (short link, unresolved)
}

// tiktokHostAllowed reports whether host is TikTok's own domain; the short
// link resolver never follows anything else.
func tiktokHostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "tiktok.com" || strings.HasSuffix(host, ".tiktok.com")
}

// parseTikTokLink extracts the first TikTok link from pasted share text.
func parseTikTokLink(text string) (tiktokLink, bool) {
	if len(text) > 2048 {
		return tiktokLink{}, false
	}
	match := tiktokLinkPattern.FindString(text)
	if match == "" {
		return tiktokLink{}, false
	}
	parsed, err := url.Parse(match)
	if err != nil || !tiktokHostAllowed(parsed.Hostname()) {
		return tiktokLink{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if groups := tiktokPostPathPattern.FindStringSubmatch(parsed.Path); groups != nil {
		return tiktokLink{raw: "https://www.tiktok.com" + parsed.Path, id: groups[2], kind: groups[1]}, true
	}
	if host == "vm.tiktok.com" || host == "vt.tiktok.com" || (host == "www.tiktok.com" && strings.HasPrefix(parsed.Path, "/t/")) {
		return tiktokLink{raw: "https://" + host + parsed.EscapedPath()}, true
	}
	return tiktokLink{}, false
}

// resolveTikTokShortLink follows at most five redirects, all of which must
// stay on TikTok hosts, and returns the canonical post link.
func resolveTikTokShortLink(ctx context.Context, client *http.Client, short string) (tiktokLink, error) {
	current := short
	for range 5 {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, current, nil)
		if err != nil {
			return tiktokLink{}, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		noFollow := *client
		noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		resp, err := noFollow.Do(req)
		if err != nil {
			return tiktokLink{}, err
		}
		_ = resp.Body.Close()
		location := resp.Header.Get("Location")
		if resp.StatusCode < 300 || resp.StatusCode >= 400 || location == "" {
			return tiktokLink{}, errors.New("short link did not redirect")
		}
		next, err := url.Parse(current)
		if err != nil {
			return tiktokLink{}, err
		}
		target, err := next.Parse(location)
		if err != nil || !tiktokHostAllowed(target.Hostname()) {
			return tiktokLink{}, errors.New("short link left TikTok")
		}
		if link, ok := parseTikTokLink(target.String()); ok && link.id != "" {
			return link, nil
		}
		current = target.String()
	}
	return tiktokLink{}, errors.New("short link redirect limit")
}

// ---------------------------------------------------------------- signing

func tiktokToolSign(parts ...string) string {
	mac := hmac.New(sha256.New, []byte("tiktok-tool:"+common.SessionSecret))
	mac.Write([]byte(strings.Join(parts, "\x1f")))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// tiktokToolClientIP returns the visitor address. In production the request
// path is Cloudflare -> Caddy (compose network) -> gateway and the origin
// firewall only admits Cloudflare, so CF-Connecting-IP is trusted only when the
// direct peer is a private/loopback proxy hop. Elsewhere it falls back to Gin.
func tiktokToolClientIP(c *gin.Context) string {
	peer := net.ParseIP(c.RemoteIP())
	if peer != nil && (peer.IsPrivate() || peer.IsLoopback()) {
		if forwarded := net.ParseIP(strings.TrimSpace(c.GetHeader("CF-Connecting-IP"))); forwarded != nil {
			return forwarded.String()
		}
	}
	return c.ClientIP()
}

// tiktokToolNetwork groups a client into its /24 (IPv4) or /48 (IPv6) so a
// pass survives mobile address churn but not a copy to another network.
func tiktokToolNetwork(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String()
	}
	return parsed.Mask(net.CIDRMask(48, 128)).String()
}

func tiktokToolAgent(c *gin.Context) string {
	sum := sha256.Sum256([]byte(c.Request.UserAgent()))
	return hex.EncodeToString(sum[:8])
}

func issueTikTokToolPass(c *gin.Context, now time.Time) {
	expires := strconv.FormatInt(now.Add(tiktokToolPassTTL).Unix(), 10)
	value := expires + "." + tiktokToolSign("pass", expires, tiktokToolNetwork(tiktokToolClientIP(c)), tiktokToolAgent(c))
	http.SetCookie(c.Writer, &http.Cookie{
		Name: tiktokToolPassCookie, Value: value, Path: "/api/tools/tiktok",
		MaxAge: int(tiktokToolPassTTL.Seconds()), HttpOnly: true, Secure: c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
	})
}

func tiktokToolPassValid(c *gin.Context, now time.Time) bool {
	raw, err := c.Cookie(tiktokToolPassCookie)
	if err != nil {
		return false
	}
	expires, signature, found := strings.Cut(raw, ".")
	if !found {
		return false
	}
	unix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || now.Unix() >= unix || unix > now.Add(tiktokToolPassTTL+time.Minute).Unix() {
		return false
	}
	expected := tiktokToolSign("pass", expires, tiktokToolNetwork(tiktokToolClientIP(c)), tiktokToolAgent(c))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// tiktokToolMediaPath signs one upstream media URL for this client.
func tiktokToolMediaPath(c *gin.Context, now time.Time, kind, upstream, filename string) string {
	expires := strconv.FormatInt(now.Add(tiktokToolMediaTTL).Unix(), 10)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(upstream))
	signature := tiktokToolSign("media", kind, encoded, filename, expires, tiktokToolNetwork(tiktokToolClientIP(c)))
	query := url.Values{"u": {encoded}, "n": {filename}, "e": {expires}, "s": {signature}}
	return "/api/tools/tiktok/file/" + kind + "?" + query.Encode()
}

// tiktokMediaHostAllowed keeps the media proxy from becoming an open proxy.
func tiktokMediaHostAllowed(host string) bool {
	host = strings.ToLower(host)
	for _, suffix := range []string{".tiktokcdn.com", ".tiktokcdn-us.com", ".tiktokcdn-eu.com", ".tiktokv.com", ".tiktokv.us", ".tiktokv.eu", ".byteoversea.com", ".ibytedtos.com", ".ibyteimg.com", ".muscdn.com"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- state

// tiktokToolCounter is a fixed-window counter backed by Redis when enabled and
// by process memory otherwise. Memory counters are per process; production
// runs Redis, and the global budget is a ceiling, not an exact meter.
type tiktokToolCounter struct {
	mu     sync.Mutex
	values map[string]tiktokToolCount
}

type tiktokToolCount struct {
	n       int64
	expires time.Time
}

var tiktokToolCounters = &tiktokToolCounter{values: map[string]tiktokToolCount{}}

func (s *tiktokToolCounter) get(ctx context.Context, key string) (int64, error) {
	if common.RedisEnabled && common.RDB != nil {
		value, err := common.RDB.Get(ctx, key).Int64()
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return value, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.values[key]
	if time.Now().After(entry.expires) {
		return 0, nil
	}
	return entry.n, nil
}

// incr adds one and returns the new value; the window starts on first use.
func (s *tiktokToolCounter) incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if common.RedisEnabled && common.RDB != nil {
		value, err := common.RDB.Incr(ctx, key).Result()
		if err != nil {
			return 0, err
		}
		if value == 1 {
			common.RDB.Expire(ctx, key, ttl)
		}
		return value, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	entry := s.values[key]
	if now.After(entry.expires) {
		entry = tiktokToolCount{expires: now.Add(ttl)}
	}
	entry.n++
	s.values[key] = entry
	if len(s.values) > 50000 {
		for k, v := range s.values {
			if now.After(v.expires) {
				delete(s.values, k)
			}
		}
	}
	return entry.n, nil
}

// setOnce stores a key only if absent (single use); true when stored.
func (s *tiktokToolCounter) setOnce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if common.RedisEnabled && common.RDB != nil {
		return common.RDB.SetNX(ctx, key, "1", ttl).Result()
	}
	value, err := s.incr(ctx, key, ttl)
	return value == 1, err
}

type tiktokToolCacheEntry struct {
	body    tiktokPost
	expires time.Time
}

var tiktokToolCache = struct {
	sync.Mutex
	entries map[string]tiktokToolCacheEntry
}{entries: map[string]tiktokToolCacheEntry{}}

// tiktokToolDownloads holds one in-flight media download per client network.
var tiktokToolDownloads sync.Map

func tiktokToolDay(now time.Time) string { return now.UTC().Format("20060102") }

func tiktokToolSecondsToMidnight(now time.Time) time.Duration {
	utc := now.UTC()
	next := time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
	return next.Sub(utc) + time.Minute
}

// ---------------------------------------------------------------- upstream

type tiktokImage struct {
	width, height int
	url           string
}

type tiktokPost struct {
	id, kind, uniqueID, nickname, description, cover string
	createdAt                                        int64
	durationMs, width, height                        int
	videoURL                                         string
	images                                           []tiktokImage
}

// tiktokPostUpstream calls the dedicated tiktok-video channel route and maps
// the TikHub aweme_detail payload. Only the fields the page renders are kept.
var tiktokPostUpstream = func(ctx context.Context, channelID int, awemeID string) (tiktokPost, int, error) {
	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil || channel.Status != common.ChannelStatusEnabled || !strings.Contains(","+channel.Models+",", ","+tiktokToolModel+",") {
		return tiktokPost{}, 0, errors.New("tiktok tool channel unavailable")
	}
	settings := channel.GetOtherSettings()
	if settings.AdvancedCustom == nil {
		return tiktokPost{}, 0, errors.New("tiktok tool channel has no routes")
	}
	path := ""
	authName, authValue := "Authorization", "Bearer {api_key}"
	for _, route := range settings.AdvancedCustom.Routes {
		if len(route.Models) == 1 && route.Models[0] == tiktokToolModel {
			path = route.UpstreamPath
			if route.Auth != nil && route.Auth.Name != "" {
				authName, authValue = route.Auth.Name, route.Auth.Value
			}
		}
	}
	if path != "/api/v1/tiktok/app/v3/fetch_one_video" {
		return tiktokPost{}, 0, errors.New("tiktok tool route mismatch")
	}
	key, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return tiktokPost{}, 0, errors.New("tiktok tool channel key unavailable")
	}
	endpoint := strings.TrimRight(channel.GetBaseURL(), "/") + path + "?aweme_id=" + url.QueryEscape(awemeID)
	ctx, cancel := context.WithTimeout(ctx, tiktokToolUpstreamTTL)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return tiktokPost{}, 0, err
	}
	req.Header.Set(authName, strings.ReplaceAll(authValue, "{api_key}", key))
	resp, err := tiktokToolFixedClient().Do(req)
	if err != nil {
		return tiktokPost{}, 0, errors.New("tiktok upstream request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return tiktokPost{}, resp.StatusCode, errors.New("tiktok upstream read failed")
	}
	if resp.StatusCode != http.StatusOK {
		return tiktokPost{}, resp.StatusCode, fmt.Errorf("tiktok upstream status %d", resp.StatusCode)
	}
	post, ok := mapTikTokPost(gjson.ParseBytes(body), awemeID)
	if !ok {
		return tiktokPost{}, http.StatusNotFound, errors.New("tiktok post unavailable")
	}
	return post, http.StatusOK, nil
}

// tiktokToolHTTPClient fetches hosts derived from user input (TikTok short
// links and TikTok CDN media). They are allowlisted, and dialing still goes
// through the SSRF-protected client as defence in depth.
var tiktokToolHTTPClient = func() *http.Client {
	return service.GetSSRFProtectedHTTPClient()
}

// tiktokToolFixedClient reaches operator-fixed endpoints only (the configured
// channel base URL and Cloudflare siteverify), like other relay calls.
var tiktokToolFixedClient = func() *http.Client {
	return service.GetHttpClient()
}

// mapTikTokPost reads one aweme_detail. A missing detail, a different id, or
// a post with neither a playable video nor images is "unavailable".
func mapTikTokPost(root gjson.Result, awemeID string) (tiktokPost, bool) {
	detail := root.Get("data.aweme_detail")
	if !detail.IsObject() {
		detail = root.Get("data.aweme_details.0")
	}
	if !detail.IsObject() || detail.Get("aweme_id").String() != awemeID {
		return tiktokPost{}, false
	}
	post := tiktokPost{
		id:          awemeID,
		uniqueID:    detail.Get("author.unique_id").String(),
		nickname:    detail.Get("author.nickname").String(),
		description: detail.Get("desc").String(),
		createdAt:   detail.Get("create_time").Int(),
		cover:       firstHTTPURL(detail.Get("video.origin_cover.url_list"), detail.Get("video.cover.url_list")),
	}
	images := detail.Get("image_post_info.images")
	if images.IsArray() && len(images.Array()) > 0 {
		post.kind = "photo"
		for _, image := range images.Array() {
			if len(post.images) >= tiktokToolMaxImages {
				break
			}
			src := firstHTTPURL(image.Get("display_image.url_list"), image.Get("owner_watermark_image.url_list"))
			if src == "" {
				continue
			}
			post.images = append(post.images, tiktokImage{
				width: int(image.Get("display_image.width").Int()), height: int(image.Get("display_image.height").Int()), url: src,
			})
		}
		if post.cover == "" && len(post.images) > 0 {
			post.cover = post.images[0].url
		}
		return post, len(post.images) > 0
	}
	post.kind = "video"
	post.durationMs = int(detail.Get("video.duration").Int())
	post.width = int(detail.Get("video.width").Int())
	post.height = int(detail.Get("video.height").Int())
	// play_addr is the clean (no end-card watermark) stream on the app API;
	// download_addr carries TikTok's watermark and is only a fallback.
	post.videoURL = firstHTTPURL(detail.Get("video.play_addr.url_list"), detail.Get("video.bit_rate.0.play_addr.url_list"), detail.Get("video.download_addr.url_list"))
	return post, post.videoURL != ""
}

func firstHTTPURL(lists ...gjson.Result) string {
	for _, list := range lists {
		for _, item := range list.Array() {
			parsed, err := url.Parse(item.String())
			if err == nil && parsed.Scheme == "https" && tiktokMediaHostAllowed(parsed.Hostname()) {
				return item.String()
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------- handlers

func tiktokToolFail(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": code})
}

// tiktokToolSameOrigin rejects cross-site callers: the API exists only for
// the tool pages, not as a free public endpoint.
func tiktokToolSameOrigin(c *gin.Context, settings tiktokToolSettings) bool {
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = c.GetHeader("Referer")
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == strings.ToLower(c.Request.Host) || strings.HasPrefix(strings.ToLower(c.Request.Host), host+":") {
		return true
	}
	for _, allowed := range settings.allowedHosts {
		if host == allowed {
			return true
		}
	}
	return false
}

func tiktokToolQuotaKey(c *gin.Context, now time.Time) (string, int) {
	settings := tiktokToolConfig()
	if userID := c.GetInt("id"); userID > 0 {
		return fmt.Sprintf("tiktoktool:v1:quota:user:%d:%s", userID, tiktokToolDay(now)), settings.userDaily
	}
	return "tiktoktool:v1:quota:ip:" + tiktokToolClientIP(c) + ":" + tiktokToolDay(now), settings.anonDaily
}

// GetTikTokToolConfig GET /api/tools/tiktok/config
func GetTikTokToolConfig(c *gin.Context) {
	settings := tiktokToolConfig()
	now := time.Now()
	key, limit := tiktokToolQuotaKey(c, now)
	used, err := tiktokToolCounters.get(c.Request.Context(), key)
	if err != nil {
		used = 0
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"enabled":            settings.enabled && settings.channelID > 0,
		"turnstile_site_key": settings.siteKey,
		"pass_valid":         tiktokToolPassValid(c, now),
		"quota":              gin.H{"used": min(used, int64(limit)), "limit": limit},
		"logged_in":          c.GetInt("id") > 0,
	}})
}

// tiktokTurnstileVerify is replaceable in tests; it calls Cloudflare siteverify.
var tiktokTurnstileVerify = func(ctx context.Context, secret, token, remoteIP string) (success bool, hostname string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	form := url.Values{"secret": {secret}, "response": {token}, "remoteip": {remoteIP}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := tiktokToolFixedClient().Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil || resp.StatusCode != http.StatusOK {
		return false, "", errors.New("siteverify unavailable")
	}
	result := gjson.ParseBytes(body)
	return result.Get("success").Bool(), result.Get("hostname").String(), nil
}

// VerifyTikTokToolTurnstile POST /api/tools/tiktok/verify
func VerifyTikTokToolTurnstile(c *gin.Context) {
	settings := tiktokToolConfig()
	if !tiktokToolSameOrigin(c, settings) {
		tiktokToolFail(c, http.StatusForbidden, "forbidden")
		return
	}
	if settings.siteKey == "" || settings.secretKey == "" {
		tiktokToolFail(c, http.StatusServiceUnavailable, "turnstile_unavailable")
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := common.DecodeJson(io.LimitReader(c.Request.Body, 8<<10), &body); err != nil || body.Token == "" || len(body.Token) > 4096 {
		tiktokToolFail(c, http.StatusBadRequest, "turnstile_failed")
		return
	}
	// Single use: a replayed token is rejected before reaching Cloudflare.
	digest := sha256.Sum256([]byte(body.Token))
	fresh, err := tiktokToolCounters.setOnce(c.Request.Context(), "tiktoktool:v1:ts:"+hex.EncodeToString(digest[:]), 10*time.Minute)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "turnstile_unavailable")
		return
	}
	if !fresh {
		tiktokToolFail(c, http.StatusForbidden, "turnstile_failed")
		return
	}
	success, hostname, err := tiktokTurnstileVerify(c.Request.Context(), settings.secretKey, body.Token, tiktokToolClientIP(c))
	if err != nil {
		common.SysError(common.LogText("tiktok tool turnstile verification unavailable: %v", err))
		tiktokToolFail(c, http.StatusServiceUnavailable, "turnstile_unavailable")
		return
	}
	hostOK := false
	for _, allowed := range settings.allowedHosts {
		if strings.EqualFold(hostname, allowed) {
			hostOK = true
		}
	}
	if !success || !hostOK {
		tiktokToolFail(c, http.StatusForbidden, "turnstile_failed")
		return
	}
	issueTikTokToolPass(c, time.Now())
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pass_valid": true}})
}

// ResolveTikTokTool POST /api/tools/tiktok/resolve
func ResolveTikTokTool(c *gin.Context) {
	settings := tiktokToolConfig()
	ctx := c.Request.Context()
	now := time.Now()
	c.Header("Cache-Control", "no-store")
	if !settings.enabled || settings.channelID <= 0 {
		tiktokToolFail(c, http.StatusServiceUnavailable, "unavailable_service")
		return
	}
	if !tiktokToolSameOrigin(c, settings) {
		tiktokToolFail(c, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		URL  string `json:"url"`
		Kind string `json:"kind"`
	}
	if err := common.DecodeJson(io.LimitReader(c.Request.Body, 8<<10), &body); err != nil || (body.Kind != "video" && body.Kind != "photo") {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_link")
		return
	}
	link, ok := parseTikTokLink(body.URL)
	if !ok {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_link")
		return
	}
	if !tiktokToolPassValid(c, now) {
		tiktokToolFail(c, http.StatusForbidden, "verification_required")
		return
	}
	burst, err := tiktokToolCounters.incr(ctx, "tiktoktool:v1:burst:"+tiktokToolClientIP(c), time.Duration(settings.burstSeconds)*time.Second)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return
	}
	if burst > 1 {
		c.Header("Retry-After", strconv.FormatInt(settings.burstSeconds, 10))
		tiktokToolFail(c, http.StatusTooManyRequests, "rate_limited")
		return
	}
	quotaKey, limit := tiktokToolQuotaKey(c, now)
	used, err := tiktokToolCounters.get(ctx, quotaKey)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return
	}
	if used >= int64(limit) {
		tiktokToolFail(c, http.StatusTooManyRequests, "quota_exceeded")
		return
	}
	if link.id == "" {
		resolved, err := resolveTikTokShortLink(ctx, tiktokToolHTTPClient(), link.raw)
		if err != nil {
			tiktokToolFail(c, http.StatusBadRequest, "invalid_link")
			return
		}
		link = resolved
	}
	if link.kind != body.Kind {
		tiktokToolFail(c, http.StatusConflict, "wrong_kind_"+link.kind)
		return
	}

	tiktokToolCache.Lock()
	cached, hit := tiktokToolCache.entries[link.id]
	tiktokToolCache.Unlock()
	post := cached.body
	if !hit || now.After(cached.expires) {
		anonymous := c.GetInt("id") == 0
		if anonymous {
			spent, err := tiktokToolCounters.get(ctx, "tiktoktool:v1:global:"+tiktokToolDay(now))
			if err != nil || spent >= int64(settings.globalAnonDaily) {
				tiktokToolFail(c, http.StatusTooManyRequests, "busy")
				return
			}
		}
		var status int
		post, status, err = tiktokPostUpstream(ctx, settings.channelID, link.id)
		if err != nil {
			if status == http.StatusNotFound {
				tiktokToolFail(c, http.StatusNotFound, "unavailable")
				return
			}
			common.SysError(common.LogText("tiktok tool upstream failed: %v", err))
			tiktokToolFail(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		if anonymous {
			_, _ = tiktokToolCounters.incr(ctx, "tiktoktool:v1:global:"+tiktokToolDay(now), tiktokToolSecondsToMidnight(now))
		}
		tiktokToolCache.Lock()
		if len(tiktokToolCache.entries) > 5000 {
			tiktokToolCache.entries = map[string]tiktokToolCacheEntry{}
		}
		tiktokToolCache.entries[link.id] = tiktokToolCacheEntry{body: post, expires: now.Add(tiktokToolCacheTTL)}
		tiktokToolCache.Unlock()
	}
	if post.kind != body.Kind {
		tiktokToolFail(c, http.StatusConflict, "wrong_kind_"+post.kind)
		return
	}
	used, err = tiktokToolCounters.incr(ctx, quotaKey, tiktokToolSecondsToMidnight(now))
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return
	}

	handle := post.uniqueID
	if handle == "" {
		handle = "tiktok"
	}
	data := gin.H{
		"kind": post.kind, "id": post.id,
		"author":      gin.H{"unique_id": post.uniqueID, "nickname": post.nickname},
		"description": post.description, "created_at": post.createdAt,
		"quota": gin.H{"used": min(used, int64(limit)), "limit": limit},
	}
	if post.cover != "" {
		data["cover_url"] = tiktokToolMediaPath(c, now, "image", post.cover, handle+"-"+post.id+"-cover"+tiktokImageExt(post.cover))
	}
	if post.kind == "video" {
		videoPath := tiktokToolMediaPath(c, now, "video", post.videoURL, handle+"-"+post.id+".mp4")
		data["video"] = gin.H{
			"duration_ms": post.durationMs, "width": post.width, "height": post.height,
			"preview_url": videoPath, "download_url": videoPath + "&dl=1",
		}
	} else {
		images := make([]gin.H, 0, len(post.images))
		for i, image := range post.images {
			name := fmt.Sprintf("%s-%s-%02d%s", handle, post.id, i+1, tiktokImageExt(image.url))
			path := tiktokToolMediaPath(c, now, "image", image.url, name)
			images = append(images, gin.H{"index": i + 1, "width": image.width, "height": image.height, "preview_url": path, "download_url": path + "&dl=1"})
		}
		data["images"] = images
		if c.GetInt("id") > 0 {
			expires := strconv.FormatInt(now.Add(tiktokToolMediaTTL).Unix(), 10)
			signature := tiktokToolSign("zip", post.id, expires, strconv.Itoa(c.GetInt("id")))
			data["zip_url"] = "/api/tools/tiktok/zip?" + url.Values{"id": {post.id}, "e": {expires}, "s": {signature}}.Encode()
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// tiktokToolOpenMedia validates a signed media link and returns the upstream
// response. The caller owns closing the body.
func tiktokToolOpenMedia(c *gin.Context, kind string, values url.Values) (*http.Response, string, int, error) {
	encoded, filename, expires, signature := values.Get("u"), values.Get("n"), values.Get("e"), values.Get("s")
	expected := tiktokToolSign("media", kind, encoded, filename, expires, tiktokToolNetwork(tiktokToolClientIP(c)))
	unix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || time.Now().Unix() >= unix || !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, "", http.StatusForbidden, errors.New("expired")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", http.StatusBadRequest, err
	}
	target, err := url.Parse(string(raw))
	if err != nil || target.Scheme != "https" || !tiktokMediaHostAllowed(target.Hostname()) {
		return nil, "", http.StatusBadRequest, errors.New("host")
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", http.StatusBadRequest, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://www.tiktok.com/")
	// Players seek with byte ranges; forward a single well-formed range only.
	if rangeHeader := c.GetHeader("Range"); strings.HasPrefix(rangeHeader, "bytes=") && !strings.Contains(rangeHeader, ",") && len(rangeHeader) < 64 {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := tiktokToolHTTPClient().Do(req)
	if err != nil {
		return nil, "", http.StatusBadGateway, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		_ = resp.Body.Close()
		return nil, "", http.StatusBadGateway, fmt.Errorf("media status %d", resp.StatusCode)
	}
	return resp, filename, http.StatusOK, nil
}

var tiktokFilenamePattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// tiktokImageExt keeps the format TikTok serves (photo posts are usually
// WebP or JPEG) so a saved file opens with the right application.
func tiktokImageExt(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ".jpg"
	}
	path := strings.ToLower(parsed.Path)
	for _, ext := range []string{".webp", ".jpeg", ".jpg", ".png", ".heic"} {
		if strings.HasSuffix(path, ext) {
			return ext
		}
	}
	return ".jpg"
}

// GetTikTokToolFile GET /api/tools/tiktok/file/:kind
func GetTikTokToolFile(c *gin.Context) {
	kind := c.Param("kind")
	if kind != "video" && kind != "image" {
		c.Status(http.StatusNotFound)
		return
	}
	network := tiktokToolNetwork(tiktokToolClientIP(c))
	attachment := c.Query("dl") == "1"
	// Only full downloads hold the per-network slot; inline playback issues
	// many short range requests and must not block the download button.
	if kind == "video" && attachment {
		if _, busy := tiktokToolDownloads.LoadOrStore(network, struct{}{}); busy {
			c.Header("Retry-After", "5")
			c.Status(http.StatusTooManyRequests)
			return
		}
		defer tiktokToolDownloads.Delete(network)
	}
	resp, filename, status, err := tiktokToolOpenMedia(c, kind, c.Request.URL.Query())
	if err != nil {
		c.Status(status)
		return
	}
	defer resp.Body.Close()
	limit := int64(tiktokToolMaxImage)
	contentType := "image/jpeg"
	if kind == "video" {
		limit, contentType = tiktokToolMaxVideo, "video/mp4"
	} else if upstreamType := resp.Header.Get("Content-Type"); strings.HasPrefix(upstreamType, "image/") {
		contentType = upstreamType
	}
	if resp.ContentLength > limit {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}
	filename = tiktokFilenamePattern.ReplaceAllString(filename, "_")
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition+`; filename="`+filename+`"`)
	c.Header("Cache-Control", "private, max-age=600")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'")
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	if kind == "video" {
		c.Header("Accept-Ranges", "bytes")
		if contentRange := resp.Header.Get("Content-Range"); resp.StatusCode == http.StatusPartialContent && contentRange != "" {
			c.Header("Content-Range", contentRange)
		}
	}
	c.Header("Content-Type", contentType)
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, io.LimitReader(resp.Body, limit))
}

// GetTikTokToolZip GET /api/tools/tiktok/zip — signed-in users only.
func GetTikTokToolZip(c *gin.Context) {
	userID := c.GetInt("id")
	id, expires, signature := c.Query("id"), c.Query("e"), c.Query("s")
	unix, err := strconv.ParseInt(expires, 10, 64)
	if userID <= 0 || err != nil || time.Now().Unix() >= unix || !hmac.Equal([]byte(signature), []byte(tiktokToolSign("zip", id, expires, strconv.Itoa(userID)))) {
		c.Status(http.StatusForbidden)
		return
	}
	tiktokToolCache.Lock()
	cached, hit := tiktokToolCache.entries[id]
	tiktokToolCache.Unlock()
	if !hit || cached.body.kind != "photo" {
		c.Status(http.StatusGone)
		return
	}
	network := tiktokToolNetwork(tiktokToolClientIP(c))
	if _, busy := tiktokToolDownloads.LoadOrStore(network, struct{}{}); busy {
		c.Header("Retry-After", "5")
		c.Status(http.StatusTooManyRequests)
		return
	}
	defer tiktokToolDownloads.Delete(network)
	handle := tiktokFilenamePattern.ReplaceAllString(cached.body.uniqueID, "_")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.zip"`, handle, id))
	c.Header("Cache-Control", "no-store")
	archive := zip.NewWriter(c.Writer)
	for i, image := range cached.body.images {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, image.url, nil)
		if err != nil {
			break
		}
		req.Header.Set("Referer", "https://www.tiktok.com/")
		resp, err := tiktokToolHTTPClient().Do(req)
		if err != nil {
			break
		}
		if resp.StatusCode == http.StatusOK {
			if entry, err := archive.Create(fmt.Sprintf("%s-%s-%02d%s", handle, id, i+1, tiktokImageExt(image.url))); err == nil {
				_, _ = io.Copy(entry, io.LimitReader(resp.Body, tiktokToolMaxImage))
			}
		}
		_ = resp.Body.Close()
	}
	_ = archive.Close()
}
