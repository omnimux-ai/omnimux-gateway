package controller

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// tiktokToolTestEnv isolates config, counters, cache and upstream for one test.
func tiktokToolTestEnv(t *testing.T) *int {
	t.Helper()
	t.Setenv("TIKTOK_TOOL_ENABLED", "true")
	t.Setenv("TIKTOK_TOOL_CHANNEL_ID", "7")
	t.Setenv("TIKTOK_TOOL_TURNSTILE_SITE_KEY", "site")
	t.Setenv("TIKTOK_TOOL_TURNSTILE_SECRET_KEY", "secret")
	t.Setenv("TIKTOK_TOOL_ANON_DAILY", "2")
	t.Setenv("TIKTOK_TOOL_BURST_SECONDS", "1")
	redisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	tiktokToolCounters = &tiktokToolCounter{values: map[string]tiktokToolCount{}}
	tiktokToolCache.entries = map[string]tiktokToolCacheEntry{}
	tiktokToolAICache.entries = map[string]tiktokToolAICacheEntry{}
	calls := 0
	upstream, verify := tiktokPostUpstream, tiktokTurnstileVerify
	tiktokPostUpstream = func(_ context.Context, _ int, id string) (tiktokPost, int, error) {
		calls++
		if id == "7400000000000000001" {
			return tiktokPost{}, http.StatusNotFound, assert.AnError
		}
		if id == "7671378797177998623" {
			return tiktokPost{id: id, kind: "photo", uniqueID: "californiacore", images: []tiktokImage{{url: "https://p19-common-sign.tiktokcdn-eu.com/a.webp"}}}, http.StatusOK, nil
		}
		return tiktokPost{id: id, kind: "video", uniqueID: "mythicshadowws", videoURL: "https://v77-perf.tiktokcdn-eu.com/v.mp4", durationMs: 15000}, http.StatusOK, nil
	}
	tiktokTurnstileVerify = func(_ context.Context, _, token, _ string) (bool, string, error) {
		switch token {
		case "down":
			return false, "", assert.AnError
		case "other-host":
			return true, "evil.example", nil
		case "bot":
			return false, "omnimux.ai", nil
		}
		return true, "omnimux.ai", nil
	}
	t.Cleanup(func() {
		tiktokPostUpstream, tiktokTurnstileVerify, common.RedisEnabled = upstream, verify, redisEnabled
	})
	return &calls
}

func tiktokToolRequest(t *testing.T, handler gin.HandlerFunc, method, target, body string, cookies []*http.Cookie, userID int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.RemoteAddr = "198.51.100.23:4000"
	ctx.Request.Host = "omnimux.ai"
	ctx.Request.Header.Set("Origin", "https://omnimux.ai")
	ctx.Request.Header.Set("User-Agent", "test-browser")
	for _, cookie := range cookies {
		ctx.Request.AddCookie(cookie)
	}
	if userID > 0 {
		ctx.Set("id", userID)
	}
	handler(ctx)
	return recorder
}

func tiktokToolPass(t *testing.T) []*http.Cookie {
	t.Helper()
	recorder := tiktokToolRequest(t, VerifyTikTokToolTurnstile, http.MethodPost, "/api/tools/tiktok/verify", `{"token":"ok-`+t.Name()+`"}`, nil, 0)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, int(tiktokToolPassTTL.Seconds()), cookies[0].MaxAge)
	return cookies
}

// tiktokToolResetBurst ends the per-IP burst window without waiting for it.
func tiktokToolResetBurst() {
	tiktokToolCounters.mu.Lock()
	delete(tiktokToolCounters.values, "tiktoktool:v1:burst:198.51.100.23")
	tiktokToolCounters.mu.Unlock()
}

func tiktokToolCode(recorder *httptest.ResponseRecorder) string {
	return gjson.GetBytes(recorder.Body.Bytes(), "code").String()
}

func TestParseTikTokLinkAcceptsShareTextAndRejectsForeignHosts(t *testing.T) {
	cases := []struct {
		input, id, kind string
		ok              bool
	}{
		{"https://www.tiktok.com/@mythicshadowws/video/7470610564927032609?is_from_webapp=1", "7470610564927032609", "video", true},
		{"看看 https://www.tiktok.com/@californiacore/photo/7671378797177998623?x=1 太好看", "7671378797177998623", "photo", true},
		{"https://vm.tiktok.com/ZMh8xYk2Q/", "", "", true},
		{"https://www.tiktok.com.evil.example/@a/video/7470610564927032609", "", "", false},
		{"https://www.youtube.com/watch?v=abc", "", "", false},
		{"https://www.tiktok.com/@a", "", "", false},
	}
	for _, tc := range cases {
		link, ok := parseTikTokLink(tc.input)
		assert.Equal(t, tc.ok, ok, tc.input)
		assert.Equal(t, tc.id, link.id, tc.input)
		assert.Equal(t, tc.kind, link.kind, tc.input)
	}
}

func TestTikTokToolResolveRequiresTurnstilePassFirst(t *testing.T) {
	calls := tiktokToolTestEnv(t)
	body := `{"url":"https://www.tiktok.com/@mythicshadowws/video/7470610564927032609","kind":"video"}`

	recorder := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", body, nil, 0)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, "verification_required", tiktokToolCode(recorder))
	assert.Zero(t, *calls, "no upstream call without a pass")

	recorder = tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", body, tiktokToolPass(t), 0)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "video", gjson.GetBytes(recorder.Body.Bytes(), "data.kind").String())
	download := gjson.GetBytes(recorder.Body.Bytes(), "data.video.download_url").String()
	assert.True(t, strings.HasPrefix(download, "/api/tools/tiktok/file/video?"), download)
	assert.NotContains(t, recorder.Body.String(), "tiktokcdn", "signed CDN URLs never reach the browser")
	assert.False(t, gjson.GetBytes(recorder.Body.Bytes(), "data.quota").Exists(), "quota counts must stay private")
	assert.False(t, gjson.GetBytes(recorder.Body.Bytes(), "data.quota_exhausted").Bool())
}

func TestTikTokToolTurnstileFailsClosedAndRejectsReplay(t *testing.T) {
	tiktokToolTestEnv(t)
	cases := []struct {
		token, code string
		status      int
	}{
		{"down", "turnstile_unavailable", http.StatusServiceUnavailable},
		{"bot", "turnstile_failed", http.StatusForbidden},
		{"other-host", "turnstile_failed", http.StatusForbidden},
	}
	for _, tc := range cases {
		recorder := tiktokToolRequest(t, VerifyTikTokToolTurnstile, http.MethodPost, "/api/tools/tiktok/verify", `{"token":"`+tc.token+`"}`, nil, 0)
		assert.Equal(t, tc.status, recorder.Code, tc.token)
		assert.Equal(t, tc.code, tiktokToolCode(recorder), tc.token)
		assert.Empty(t, recorder.Result().Cookies(), tc.token)
	}

	first := tiktokToolRequest(t, VerifyTikTokToolTurnstile, http.MethodPost, "/api/tools/tiktok/verify", `{"token":"once"}`, nil, 0)
	require.Equal(t, http.StatusOK, first.Code)
	replay := tiktokToolRequest(t, VerifyTikTokToolTurnstile, http.MethodPost, "/api/tools/tiktok/verify", `{"token":"once"}`, nil, 0)
	assert.Equal(t, http.StatusForbidden, replay.Code)

	t.Setenv("TIKTOK_TOOL_TURNSTILE_SECRET_KEY", "")
	unconfigured := tiktokToolRequest(t, VerifyTikTokToolTurnstile, http.MethodPost, "/api/tools/tiktok/verify", `{"token":"fresh"}`, nil, 0)
	assert.Equal(t, "turnstile_unavailable", tiktokToolCode(unconfigured))
}

func TestTikTokToolPassIsBoundToNetworkAndExpires(t *testing.T) {
	tiktokToolTestEnv(t)
	cookies := tiktokToolPass(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request.Header.Set("User-Agent", "test-browser")
	ctx.Request.AddCookie(cookies[0])

	ctx.Request.RemoteAddr = "198.51.100.200:1"
	assert.True(t, tiktokToolPassValid(ctx, time.Now()), "same /24 keeps the pass")
	ctx.Request.RemoteAddr = "203.0.113.5:1"
	assert.False(t, tiktokToolPassValid(ctx, time.Now()), "another network cannot reuse a copied cookie")
	ctx.Request.RemoteAddr = "198.51.100.23:1"
	assert.False(t, tiktokToolPassValid(ctx, time.Now().Add(tiktokToolPassTTL+time.Second)), "the pass expires after two hours")
}

func TestTikTokToolQuotaCacheAndErrorsDoNotConsumeQuota(t *testing.T) {
	calls := tiktokToolTestEnv(t)
	cookies := tiktokToolPass(t)
	resolve := func(url, kind string) *httptest.ResponseRecorder {
		tiktokToolResetBurst()
		return tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", `{"url":"`+url+`","kind":"`+kind+`"}`, cookies, 0)
	}
	video := "https://www.tiktok.com/@mythicshadowws/video/7470610564927032609"

	assert.Equal(t, "invalid_link", tiktokToolCode(resolve("https://example.com/x", "video")))
	assert.Equal(t, "wrong_kind_photo", tiktokToolCode(resolve("https://www.tiktok.com/@californiacore/photo/7671378797177998623", "video")))
	assert.Equal(t, "unavailable", tiktokToolCode(resolve("https://www.tiktok.com/@someone/video/7400000000000000001", "video")))

	cfgBefore := tiktokToolRequest(t, GetTikTokToolConfig, http.MethodGet, "/api/tools/tiktok/config", "", cookies, 0)
	require.Equal(t, http.StatusOK, cfgBefore.Code)
	assert.False(t, gjson.GetBytes(cfgBefore.Body.Bytes(), "data.quota").Exists(), "config must not leak counts before exhaustion")
	assert.False(t, gjson.GetBytes(cfgBefore.Body.Bytes(), "data.quota_exhausted").Bool(), "config must report not exhausted before daily limit")

	first := resolve(video, "video")
	require.Equal(t, http.StatusOK, first.Code)
	assert.False(t, gjson.GetBytes(first.Body.Bytes(), "data.quota_exhausted").Bool(), "failures above did not exhaust daily limit of 2")
	assert.False(t, gjson.GetBytes(first.Body.Bytes(), "data.quota").Exists())
	second := resolve(video, "video")
	require.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, 2, *calls, "the second parse of the same post is served from cache")
	assert.True(t, gjson.GetBytes(second.Body.Bytes(), "data.quota_exhausted").Bool(), "second success hits daily limit of 2")
	assert.False(t, gjson.GetBytes(second.Body.Bytes(), "data.quota").Exists())

	third := resolve(video, "video")
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.Equal(t, "quota_exceeded", tiktokToolCode(third))

	cfgRec := tiktokToolRequest(t, GetTikTokToolConfig, http.MethodGet, "/api/tools/tiktok/config", "", cookies, 0)
	require.Equal(t, http.StatusOK, cfgRec.Code)
	assert.False(t, gjson.GetBytes(cfgRec.Body.Bytes(), "data.quota").Exists(), "config must not leak counts")
	assert.True(t, gjson.GetBytes(cfgRec.Body.Bytes(), "data.quota_exhausted").Bool())

	// Default daily allowances: anonymous 5, signed-in user 10.
	t.Setenv("TIKTOK_TOOL_ANON_DAILY", "")
	t.Setenv("TIKTOK_TOOL_USER_DAILY", "")
	defaults := tiktokToolConfig()
	assert.Equal(t, 5, defaults.anonDaily)
	assert.Equal(t, 10, defaults.userDaily)
}

func TestTikTokToolRejectsCrossSiteAndBurst(t *testing.T) {
	tiktokToolTestEnv(t)
	cookies := tiktokToolPass(t)
	body := `{"url":"https://www.tiktok.com/@mythicshadowws/video/7470610564927032609","kind":"video"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/tools/tiktok/resolve", strings.NewReader(body))
	ctx.Request.Host = "omnimux.ai"
	ctx.Request.Header.Set("Origin", "https://free-api.example")
	ResolveTikTokTool(ctx)
	assert.Equal(t, "forbidden", tiktokToolCode(recorder))

	first := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", body, cookies, 0)
	require.Equal(t, http.StatusOK, first.Code)
	burst := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", body, cookies, 0)
	assert.Equal(t, http.StatusTooManyRequests, burst.Code)
	assert.Equal(t, "rate_limited", tiktokToolCode(burst))
}

func TestTikTokToolGlobalBudgetStopsAnonymousButNotSignedInUsers(t *testing.T) {
	tiktokToolTestEnv(t)
	t.Setenv("TIKTOK_TOOL_GLOBAL_ANON_DAILY", "1")
	_, err := tiktokToolCounters.incr(context.Background(), "tiktoktool:v1:global:"+tiktokToolDay(time.Now()), time.Hour)
	require.NoError(t, err)
	cookies := tiktokToolPass(t)
	body := `{"url":"https://www.tiktok.com/@mythicshadowws/video/7470610564927032609","kind":"video"}`

	anonymous := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", body, cookies, 0)
	assert.Equal(t, "busy", tiktokToolCode(anonymous))
	tiktokToolResetBurst()
	signedIn := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", body, cookies, 42)
	assert.Equal(t, http.StatusOK, signedIn.Code, signedIn.Body.String())
}

func TestTikTokToolMediaLinksAreSignedAndHostRestricted(t *testing.T) {
	tiktokToolTestEnv(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "198.51.100.23:1"
	now := time.Now()

	signed := tiktokToolMediaPath(ctx, now, "image", "https://evil.example/a.jpg", "a.jpg")
	ctx.Request = httptest.NewRequest(http.MethodGet, signed, nil)
	ctx.Request.RemoteAddr = "198.51.100.23:1"
	_, _, status, err := tiktokToolOpenMedia(ctx, "image", ctx.Request.URL.Query())
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, status, "a validly signed link still cannot reach a non-TikTok host")

	signed = tiktokToolMediaPath(ctx, now, "image", "https://p19-common-sign.tiktokcdn-eu.com/a.webp", "a.jpg")
	tampered := strings.Replace(signed, "n=a.jpg", "n=b.jpg", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, tampered, nil)
	ctx.Request.RemoteAddr = "198.51.100.23:1"
	_, _, status, _ = tiktokToolOpenMedia(ctx, "image", ctx.Request.URL.Query())
	assert.Equal(t, http.StatusForbidden, status)

	ctx.Request = httptest.NewRequest(http.MethodGet, signed, nil)
	ctx.Request.RemoteAddr = "203.0.113.9:1"
	_, _, status, _ = tiktokToolOpenMedia(ctx, "image", ctx.Request.URL.Query())
	assert.Equal(t, http.StatusForbidden, status, "media links are bound to the requesting network")
}

func TestMapTikTokPostReadsVideoAndPhotoDetails(t *testing.T) {
	video, ok := mapTikTokPost(gjson.Parse(`{"data":{"aweme_detail":{"aweme_id":"1","desc":"d","author":{"unique_id":"u"},"video":{"duration":15000,"width":720,"height":1280,"play_addr":{"url_list":["http://x.tiktokcdn.com/a","https://v16.tiktokcdn.com/clean.mp4"]},"download_addr":{"url_list":["https://v16.tiktokcdn.com/wm.mp4"]}}}}}`), "1")
	require.True(t, ok)
	assert.Equal(t, "video", video.kind)
	assert.Equal(t, "https://v16.tiktokcdn.com/clean.mp4", video.videoURL, "prefers https play_addr over the watermarked download_addr")

	photo, ok := mapTikTokPost(gjson.Parse(`{"data":{"aweme_detail":{"aweme_id":"2","image_post_info":{"images":[{"display_image":{"width":1080,"height":1440,"url_list":["https://p16.tiktokcdn.com/1.jpeg"]}},{"display_image":{"url_list":[]}}]}}}}`), "2")
	require.True(t, ok)
	assert.Equal(t, "photo", photo.kind)
	assert.Len(t, photo.images, 1)

	_, ok = mapTikTokPost(gjson.Parse(`{"data":{"aweme_detail":{"aweme_id":"3"}}}`), "2")
	assert.False(t, ok, "a different post id is unavailable")

	heicPreferJpeg, ok := mapTikTokPost(gjson.Parse(`{"data":{"aweme_detail":{"aweme_id":"4","video":{"play_addr":{"url_list":["https://v16.tiktokcdn.com/v.mp4"]},"origin_cover":{"url_list":["https://p16.tiktokcdn.com/cover.heic?x=1"]},"cover":{"url_list":["https://p16.tiktokcdn.com/cover.jpeg?x=2"]}}}}}`), "4")
	require.True(t, ok)
	assert.Equal(t, "https://p16.tiktokcdn.com/cover.jpeg?x=2", heicPreferJpeg.cover, "prefers browser-decodable image over heic")

	heicFallback, ok := mapTikTokPost(gjson.Parse(`{"data":{"aweme_detail":{"aweme_id":"5","video":{"play_addr":{"url_list":["https://v16.tiktokcdn.com/v.mp4"]},"origin_cover":{"url_list":["https://p16.tiktokcdn.com/only.heic"]}}}}}`), "5")
	require.True(t, ok)
	assert.Equal(t, "https://p16.tiktokcdn.com/only.heic", heicFallback.cover, "falls back to heic when no other format exists")

	photoWebp, ok := mapTikTokPost(gjson.Parse(`{"data":{"aweme_detail":{"aweme_id":"6","image_post_info":{"images":[{"display_image":{"width":1080,"height":1440,"url_list":["https://p16.tiktokcdn.com/1.heic","https://p16.tiktokcdn.com/1.webp"]}}]}}}}`), "6")
	require.True(t, ok)
	require.Len(t, photoWebp.images, 1)
	assert.Equal(t, "https://p16.tiktokcdn.com/1.webp", photoWebp.images[0].url)
}

func TestCopyChannelTikTokToolKeepsOnlyTheVideoRouteDisabled(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	origin := accountMonitorCopyFixture(t)
	settings := origin.GetOtherSettings()
	settings.AdvancedCustom.Routes = append(settings.AdvancedCustom.Routes, dto.AdvancedCustomRoute{
		IncomingPath: "/v1/chat/completions", UpstreamPath: "/api/v1/tiktok/app/v3/fetch_one_video", Converter: "none",
		Models: []string{"tiktok-video"}, UpstreamMethod: http.MethodGet, QueryMapping: map[string]string{"aweme_id": "aweme_id"},
		Auth: &dto.AdvancedCustomRouteAuth{Type: "header", Name: "Authorization", Value: "Bearer {api_key}"},
	})
	origin.SetOtherSettings(settings)
	origin.Models += ",tiktok-video"
	require.NoError(t, db.Create(&origin).Error)

	recorder := performAccountMonitorCopy(t, origin.Id, `{"scope":"tiktok-tool"}`)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var clone model.Channel
	require.NoError(t, db.First(&clone, gjson.GetBytes(recorder.Body.Bytes(), "data.id").Int()).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, clone.Status)
	assert.Equal(t, "tiktok-tool", clone.Group)
	assert.Equal(t, "tiktok-video", clone.Models)
	assert.Equal(t, origin.Key, clone.Key, "the credential is copied server-side")
	routes := clone.GetOtherSettings().AdvancedCustom.Routes
	require.Len(t, routes, 1)
	assert.Equal(t, "/api/v1/tiktok/app/v3/fetch_one_video", routes[0].UpstreamPath)
	assert.Nil(t, clone.ParamOverride)
	assert.Equal(t, constant.ChannelTypeAdvancedCustom, clone.Type)
	assert.NotContains(t, recorder.Body.String(), "fake-channel-private-value")
}

func TestTikTokToolClientIPTrustsCloudflareHeaderOnlyFromPrivateProxy(t *testing.T) {
	cases := []struct {
		peer, header, want string
	}{
		{"172.18.0.5:443", "203.0.113.7", "203.0.113.7"},
		{"198.51.100.23:443", "203.0.113.7", "198.51.100.23"},
		{"172.18.0.5:443", "not-an-ip", "172.18.0.5"},
	}
	for _, tc := range cases {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		ctx.Request.RemoteAddr = tc.peer
		ctx.Request.Header.Set("CF-Connecting-IP", tc.header)
		assert.Equal(t, tc.want, tiktokToolClientIP(ctx), tc.peer)
	}
}

func TestTikTokToolPassCookieAttributesAndSecureMatrix(t *testing.T) {
	previousSecure := common.SessionCookieSecure
	t.Cleanup(func() { common.SessionCookieSecure = previousSecure })

	cases := []struct {
		name           string
		sessionSecure  bool
		directTLS      bool
		forwardedProto string
		wantSecure     bool
	}{
		{
			name:          "HTTPS production config sets Secure without TLS or proxy headers",
			sessionSecure: true,
			wantSecure:    true,
		},
		{
			name:          "direct TLS sets Secure even when SessionCookieSecure is false",
			sessionSecure: false,
			directTLS:     true,
			wantSecure:    true,
		},
		{
			name:          "local HTTP keeps Secure false",
			sessionSecure: false,
			wantSecure:    false,
		},
		{
			name:           "local HTTP ignores forged X-Forwarded-Proto header",
			sessionSecure:  false,
			forwardedProto: "https",
			wantSecure:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tiktokToolTestEnv(t)
			common.SessionCookieSecure = tc.sessionSecure

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/tools/tiktok/verify", strings.NewReader(`{"token":"matrix-`+tc.name+`"}`))
			ctx.Request.RemoteAddr = "198.51.100.23:4000"
			ctx.Request.Host = "omnimux.ai"
			ctx.Request.Header.Set("Origin", "https://omnimux.ai")
			ctx.Request.Header.Set("User-Agent", "test-browser")
			if tc.directTLS {
				ctx.Request.TLS = &tls.ConnectionState{}
			}
			if tc.forwardedProto != "" {
				ctx.Request.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}

			VerifyTikTokToolTurnstile(ctx)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			cookies := recorder.Result().Cookies()
			require.Len(t, cookies, 1)
			cookie := cookies[0]
			assert.Equal(t, tiktokToolPassCookie, cookie.Name)
			assert.Equal(t, "/api/tools/tiktok", cookie.Path)
			assert.Empty(t, cookie.Domain)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
			assert.Equal(t, int(tiktokToolPassTTL.Seconds()), cookie.MaxAge)
			assert.Equal(t, tc.wantSecure, cookie.Secure)
		})
	}
}

func TestParseWebVTTCuesAndEdgeCases(t *testing.T) {
	raw := "\uFEFFWEBVTT\r\n\r\n" +
		"NOTE comment block ignored\r\n\r\n" +
		"STYLE\r\n::cue { color: white; }\r\n\r\n" +
		"1\r\n" +
		"00:00.250 --> 00:03.500 align:start position:0%\r\n" +
		"<00:00:00.300><c>Hello &amp; welcome</c>\r\n" +
		"to <b>OmniMux</b>\r\n\r\n" +
		"2\r\n" +
		"00:00:03,500 --> 00:00:06,750\r\n" +
		"Hello &amp; welcome to OmniMux\r\n\r\n" +
		"3\r\n" +
		"00:07.000 --> 00:10.100\r\n" +
		"Second unique line\r\n\r\n" +
		"4\r\n" +
		"-00:05.000 --> 00:08.000\r\n" +
		"Signed negative timestamp rejected\r\n"

	cues := parseWebVTT(raw)
	require.Len(t, cues, 2)
	assert.Equal(t, tiktokTranscriptCue{StartMs: 250, EndMs: 3500, Text: "Hello & welcome to OmniMux"}, cues[0])
	assert.Equal(t, tiktokTranscriptCue{StartMs: 7000, EndMs: 10100, Text: "Second unique line"}, cues[1])
	assert.False(t, tiktokCaptionHostAllowed(".tiktokcdn.com"))
}

func TestMapTikTokPostExtractsCaptionTracksAndRespectsAllowlist(t *testing.T) {
	t.Setenv("TIKTOK_TOOL_EXTRA_CAPTION_HOSTS", "subtitles.example.net")
	payload := `{
		"data": {
			"aweme_detail": {
				"aweme_id": "9001",
				"desc": "demo",
				"author": {"unique_id": "creator"},
				"video": {
					"duration": 12000,
					"width": 720,
					"height": 1280,
					"play_addr": {"url_list": ["https://v16.tiktokcdn.com/video.mp4"]},
					"cla_info": {
						"caption_infos": [
							{"lang": "eng-US", "language_code": "en", "caption_format": "webvtt", "is_auto_generated": true, "url": "https://mcdn.tiktokcdn.com/en.vtt"},
							{"lang": "eng-US", "language_code": "en", "caption_format": "webvtt", "is_auto_generated": true, "url": "https://mcdn.tiktokcdn.com/en-dup.vtt"},
							{"lang": "spa-ES", "language_code": "es", "caption_format": "webvtt", "is_auto_generated": false, "url_list": ["https://subtitles.example.net/es.vtt"]},
							{"lang": "evil", "language_code": "fr", "caption_format": "webvtt", "url": "https://evil.example/fr.vtt"},
							{"lang": "bin", "language_code": "de", "caption_format": "srt", "url": "https://mcdn.tiktokcdn.com/de.srt"}
						]
					}
				}
			}
		}
	}`
	post, ok := mapTikTokPost(gjson.Parse(payload), "9001")
	require.True(t, ok)
	require.Len(t, post.captions, 2)
	assert.Equal(t, "en", post.captions[0].languageCode)
	assert.True(t, post.captions[0].isAutoGenerated)
	assert.Equal(t, "https://mcdn.tiktokcdn.com/en.vtt", post.captions[0].url)
	assert.Equal(t, "es", post.captions[1].languageCode)
	assert.False(t, post.captions[1].isAutoGenerated)
	assert.Equal(t, "https://subtitles.example.net/es.vtt", post.captions[1].url)
}

func TestTikTokToolResolveTranscriptCachesAndFallsBackHonestly(t *testing.T) {
	calls := tiktokToolTestEnv(t)
	prevFetch := tiktokCaptionFetch
	fetchCalls := 0
	tiktokCaptionFetch = func(_ context.Context, rawURL string) (string, error) {
		fetchCalls++
		if strings.Contains(rawURL, "en.vtt") {
			return "WEBVTT\n\n00:00.000 --> 00:03.200\nOpening hook line\n", nil
		}
		return "", assert.AnError
	}
	prevUpstream := tiktokPostUpstream
	tiktokPostUpstream = func(_ context.Context, _ int, id string) (tiktokPost, int, error) {
		*calls++
		if id == "7671378797177998623" {
			return tiktokPost{id: id, kind: "photo", uniqueID: "photouser", images: []tiktokImage{{url: "https://p19.tiktokcdn-eu.com/a.webp"}}}, http.StatusOK, nil
		}
		if id == "7470610564927032699" {
			return tiktokPost{
				id: id, kind: "video", uniqueID: "nocapuser", description: "No cap video desc",
				videoURL: "https://v77-perf.tiktokcdn-eu.com/nocap.mp4", durationMs: 9000,
			}, http.StatusOK, nil
		}
		return tiktokPost{
			id: id, kind: "video", uniqueID: "capuser", description: "Captioned desc",
			videoURL: "https://v77-perf.tiktokcdn-eu.com/cap.mp4", durationMs: 15000,
			captions: []tiktokCaptionTrack{{
				lang: "eng-US", languageCode: "en", format: "webvtt", isAutoGenerated: true,
				url: "https://mcdn.tiktokcdn.com/en.vtt",
			}},
		}, http.StatusOK, nil
	}
	t.Cleanup(func() {
		tiktokCaptionFetch = prevFetch
		tiktokPostUpstream = prevUpstream
	})

	t.Setenv("TIKTOK_TOOL_ANON_DAILY", "5")
	cookies := tiktokToolPass(t)

	// 1. Captioned video returns transcript tracks + signed MP4 fallback and caches transcripts.
	capURL := "https://www.tiktok.com/@capuser/video/7470610564927032609"
	first := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", `{"url":"`+capURL+`","kind":"transcript"}`, cookies, 0)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	assert.Equal(t, "transcript", gjson.GetBytes(first.Body.Bytes(), "data.kind").String())
	assert.True(t, gjson.GetBytes(first.Body.Bytes(), "data.transcript.has_captions").Bool())
	assert.Equal(t, "Opening hook line", gjson.GetBytes(first.Body.Bytes(), "data.transcript.tracks.0.cues.0.text").String())
	assert.True(t, strings.HasPrefix(gjson.GetBytes(first.Body.Bytes(), "data.video.download_url").String(), "/api/tools/tiktok/file/video?"))
	assert.NotContains(t, first.Body.String(), "tiktokcdn")

	tiktokToolResetBurst()
	second := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", `{"url":"`+capURL+`","kind":"transcript"}`, cookies, 0)
	require.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, 1, *calls)
	assert.Equal(t, 1, fetchCalls, "cached transcript avoids re-fetching WebVTT")

	// 2. Video without native captions returns has_captions=false while keeping description & download link.
	tiktokToolResetBurst()
	noCapURL := "https://www.tiktok.com/@nocapuser/video/7470610564927032699"
	noCap := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", `{"url":"`+noCapURL+`","kind":"transcript"}`, cookies, 0)
	require.Equal(t, http.StatusOK, noCap.Code, noCap.Body.String())
	assert.False(t, gjson.GetBytes(noCap.Body.Bytes(), "data.transcript.has_captions").Bool())
	assert.Empty(t, gjson.GetBytes(noCap.Body.Bytes(), "data.transcript.tracks").Array())
	assert.Equal(t, "No cap video desc", gjson.GetBytes(noCap.Body.Bytes(), "data.description").String())

	// 3. Photo link returns wrong_kind_photo even in transcript mode.
	tiktokToolResetBurst()
	photoURL := "https://www.tiktok.com/@photouser/photo/7671378797177998623"
	photo := tiktokToolRequest(t, ResolveTikTokTool, http.MethodPost, "/api/tools/tiktok/resolve", `{"url":"`+photoURL+`","kind":"transcript"}`, cookies, 0)
	assert.Equal(t, http.StatusConflict, photo.Code)
	assert.Equal(t, "wrong_kind_photo", tiktokToolCode(photo))
}

func TestTikTokCaptionFetchGuards(t *testing.T) {
	t.Setenv("TIKTOK_TOOL_EXTRA_CAPTION_HOSTS", "captions.example.test")

	evilHits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "evil.example.test" {
			evilHits++
		}
		switch r.URL.Path {
		case "/ok.vtt":
			w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
			_, _ = w.Write([]byte("WEBVTT\n\n00:00.000 --> 00:01.000\nok\n"))
		case "/redir-evil":
			http.Redirect(w, r, "https://evil.example.test/ok.vtt", http.StatusFound)
		case "/loop":
			hop, _ := strconv.Atoi(r.URL.Query().Get("hop"))
			http.Redirect(w, r, "https://captions.example.test/loop?hop="+strconv.Itoa(hop+1), http.StatusFound)
		case "/mime":
			w.Header().Set("Content-Type", r.URL.Query().Get("ct"))
			_, _ = w.Write([]byte("WEBVTT\n\n00:00.000 --> 00:01.000\nblocked\n"))
		case "/oversize-chunked":
			w.Header().Set("Content-Type", "text/vtt")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = w.Write([]byte(strings.Repeat("a", tiktokToolMaxCaption+1)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	baseTransport := srv.Client().Transport.(*http.Transport).Clone()
	baseTransport.TLSClientConfig = baseTransport.TLSClientConfig.Clone()
	baseTransport.TLSClientConfig.InsecureSkipVerify = true
	dialer := &net.Dialer{}
	baseTransport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, srv.Listener.Addr().String())
	}

	prevClient := tiktokCaptionHTTPClient
	tiktokCaptionHTTPClient = func() *http.Client {
		return &http.Client{Transport: baseTransport, Timeout: 5 * time.Second}
	}
	t.Cleanup(func() {
		tiktokCaptionHTTPClient = prevClient
	})

	ctx := context.Background()

	body, err := tiktokCaptionFetch(ctx, "https://captions.example.test/ok.vtt")
	require.NoError(t, err)
	assert.Contains(t, body, "00:00.000 --> 00:01.000")

	_, err = tiktokCaptionFetch(ctx, "https://evil.example.test/ok.vtt")
	assert.ErrorContains(t, err, "host not allowed")

	_, err = tiktokCaptionFetch(ctx, "https://captions.example.test/redir-evil")
	assert.ErrorContains(t, err, "redirect host not allowed")

	_, err = tiktokCaptionFetch(ctx, "https://captions.example.test/loop?hop=0")
	assert.ErrorContains(t, err, "too many caption redirects")

	for _, ct := range []string{"text/html; charset=utf-8", "video/mp4", "audio/mpeg", "image/png"} {
		_, err = tiktokCaptionFetch(ctx, "https://captions.example.test/mime?ct="+url.QueryEscape(ct))
		assert.ErrorContains(t, err, "content type", "expected rejection for Content-Type %s", ct)
	}

	_, err = tiktokCaptionFetch(ctx, "https://captions.example.test/oversize-chunked")
	assert.ErrorContains(t, err, "exceeds size limit")
	assert.Zero(t, evilHits, "redirect to disallowed host must be stopped before dialing")
}

func TestTikTokToolAITranslateAndBreakdown(t *testing.T) {
	tiktokToolTestEnv(t)
	videoID := "7470610564927032609"
	noCapID := "7470610564927032699"

	// Cross-origin caller rejected before auth or LLM execution.
	crossRec := httptest.NewRecorder()
	crossCtx, _ := gin.CreateTestContext(crossRec)
	crossCtx.Request = httptest.NewRequest(http.MethodPost, "/api/tools/tiktok/translate", strings.NewReader(`{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-CN"}`))
	crossCtx.Request.Host = "omnimux.ai"
	crossCtx.Request.Header.Set("Origin", "https://evil.example")
	TranslateTikTokToolTranscript(crossCtx)
	assert.Equal(t, http.StatusForbidden, crossRec.Code)
	assert.Equal(t, "forbidden", tiktokToolCode(crossRec))

	// Anonymous user gets login_required.
	anonRes := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-CN"}`, nil, 0)
	assert.Equal(t, http.StatusUnauthorized, anonRes.Code)
	assert.Equal(t, "login_required", tiktokToolCode(anonRes))

	// Input validation & expired cache checks.
	badID := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"bad","target_lang":"zh-CN"}`, nil, 42)
	assert.Equal(t, http.StatusBadRequest, badID.Code)
	assert.Equal(t, "invalid_param", tiktokToolCode(badID))

	badLang := tiktokToolRequest(t, BreakdownTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/breakdown", `{"video_id":"`+videoID+`","target_lang":"xx"}`, nil, 42)
	assert.Equal(t, http.StatusBadRequest, badLang.Code)
	assert.Equal(t, "invalid_param", tiktokToolCode(badLang))

	missingCache := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","target_lang":"zh-CN"}`, nil, 42)
	assert.Equal(t, http.StatusGone, missingCache.Code)
	assert.Equal(t, "cache_expired", tiktokToolCode(missingCache))

	// Seed cached posts: one with captions, one without.
	now := time.Now()
	tiktokToolCache.Lock()
	tiktokToolCache.entries[noCapID] = tiktokToolCacheEntry{
		body:    tiktokPost{id: noCapID, kind: "video", transcriptsLoaded: true, transcripts: nil},
		expires: now.Add(time.Hour),
	}
	tiktokToolCache.entries[videoID] = tiktokToolCacheEntry{
		body: tiktokPost{
			id:                videoID,
			kind:              "video",
			durationMs:        12000,
			transcriptsLoaded: true,
			transcripts: []tiktokTranscriptTrack{
				{
					ID:           "en-auto",
					Lang:         "eng-US",
					LanguageCode: "en",
					Cues: []tiktokTranscriptCue{
						{StartMs: 0, EndMs: 3200, Text: "First hook line"},
						{StartMs: 3200, EndMs: 7000, Text: "Second proof step"},
					},
				},
			},
		},
		expires: now.Add(time.Hour),
	}
	tiktokToolCache.Unlock()

	noCapRes := tiktokToolRequest(t, BreakdownTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/breakdown", `{"video_id":"`+noCapID+`","target_lang":"zh-CN"}`, nil, 42)
	assert.Equal(t, http.StatusBadRequest, noCapRes.Code)
	assert.Equal(t, "no_captions", tiktokToolCode(noCapRes))

	aiCalls := 0
	failNext := false
	prevChat := service.TikTokToolAIChat
	service.TikTokToolAIChat = func(_ context.Context, _ string, _ string, system, _ string, check func(string) error) (string, error) {
		aiCalls++
		if failNext {
			failNext = false
			return "", assert.AnError
		}
		var reply string
		if strings.Contains(system, `"cues"`) {
			reply = "```json\n{\"cues\":[{\"i\":0,\"text\":\"第一句开场钩子\"},{\"i\":1,\"text\":\"第二步效果展示\"}]}\n```"
		} else {
			reply = `{"summary":"先抛出痛点再实操证明","sections":[{"stage":"hook","label":"开场钩子","start_ms":0,"end_ms":3200,"quote":"First hook line","analysis":"直接吸引停留"},{"stage":"custom_weird","label":"核心实测","start_ms":3200,"end_ms":99000,"quote":"Second proof step","analysis":"用画面证明效果"}]}`
		}
		if check != nil {
			require.NoError(t, check(reply))
		}
		return reply, nil
	}
	t.Cleanup(func() {
		service.TikTokToolAIChat = prevChat
	})

	// Translate preserves timestamps, hides numeric quota fields, and caches hits.
	tr1 := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-CN"}`, nil, 42)
	require.Equal(t, http.StatusOK, tr1.Code, tr1.Body.String())
	assert.Equal(t, 1, aiCalls)
	assert.False(t, gjson.GetBytes(tr1.Body.Bytes(), "data.quota").Exists())
	assert.Equal(t, "第一句开场钩子", gjson.GetBytes(tr1.Body.Bytes(), "data.cues.0.text").String())
	assert.Equal(t, int64(3200), gjson.GetBytes(tr1.Body.Bytes(), "data.cues.1.start_ms").Int())

	tr2 := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-CN"}`, nil, 42)
	require.Equal(t, http.StatusOK, tr2.Code)
	assert.Equal(t, 1, aiCalls, "second translate hits cache without invoking model or burst")

	// Failed model call must return 502 and must not increment user AI quota.
	failNext = true
	trFail := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"ja"}`, nil, 42)
	assert.Equal(t, http.StatusBadGateway, trFail.Code)
	assert.Equal(t, "upstream_failed", tiktokToolCode(trFail))

	aiQuotaKey := "tiktoktool:v1:ai_user:" + tiktokToolDay(time.Now()) + ":42"
	usedAfterFail, err := tiktokToolCounters.get(context.Background(), aiQuotaKey)
	require.NoError(t, err)
	assert.Equal(t, int64(1), usedAfterFail)

	// Breakdown normalizes unknown stage to "pacing", clamps end_ms to video duration, and caches hit.
	bd1 := tiktokToolRequest(t, BreakdownTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/breakdown", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-CN"}`, nil, 42)
	require.Equal(t, http.StatusOK, bd1.Code, bd1.Body.String())
	assert.Equal(t, "先抛出痛点再实操证明", gjson.GetBytes(bd1.Body.Bytes(), "data.summary").String())
	assert.Equal(t, "pacing", gjson.GetBytes(bd1.Body.Bytes(), "data.sections.1.stage").String())
	assert.Equal(t, int64(12000), gjson.GetBytes(bd1.Body.Bytes(), "data.sections.1.end_ms").Int())

	callsBeforeCache := aiCalls
	bd2 := tiktokToolRequest(t, BreakdownTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/breakdown", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-CN"}`, nil, 42)
	require.Equal(t, http.StatusOK, bd2.Code)
	assert.Equal(t, callsBeforeCache, aiCalls)

	// Enforce per-user daily AI limit on fresh user.
	t.Setenv("TIKTOK_TOOL_AI_USER_DAILY", "1")
	okUser := tiktokToolRequest(t, BreakdownTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/breakdown", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"es"}`, nil, 99)
	require.Equal(t, http.StatusOK, okUser.Code)
	overLimit := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"es"}`, nil, 99)
	assert.Equal(t, http.StatusTooManyRequests, overLimit.Code)
	assert.Equal(t, "ai_quota_exceeded", tiktokToolCode(overLimit))

	// Enforce per-user 4 req/30s burst limit without burning daily quota on the rejected 5th request.
	t.Setenv("TIKTOK_TOOL_AI_USER_DAILY", "10")
	burstDailyKey := "tiktoktool:v1:ai_user:" + tiktokToolDay(time.Now()) + ":101"
	for _, lang := range []string{"ja", "es", "id", "zh-TW"} {
		rec := tiktokToolRequest(t, TranslateTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/translate", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"`+lang+`"}`, nil, 101)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	burstHit := tiktokToolRequest(t, BreakdownTikTokToolTranscript, http.MethodPost, "/api/tools/tiktok/breakdown", `{"video_id":"`+videoID+`","track_id":"en-auto","target_lang":"zh-TW"}`, nil, 101)
	assert.Equal(t, http.StatusTooManyRequests, burstHit.Code)
	assert.Equal(t, "rate_limited", tiktokToolCode(burstHit))
	assert.Equal(t, "30", burstHit.Header().Get("Retry-After"))
	usedAfterBurst, err := tiktokToolCounters.get(context.Background(), burstDailyKey)
	require.NoError(t, err)
	assert.Equal(t, int64(4), usedAfterBurst)
}
