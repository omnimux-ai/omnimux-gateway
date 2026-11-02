package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
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
	assert.Equal(t, int64(1), gjson.GetBytes(recorder.Body.Bytes(), "data.quota.used").Int())
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

	require.Equal(t, http.StatusOK, resolve(video, "video").Code)
	second := resolve(video, "video")
	require.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, 2, *calls, "the second parse of the same post is served from cache")
	assert.Equal(t, int64(2), gjson.GetBytes(second.Body.Bytes(), "data.quota.used").Int(), "failures above did not consume quota")

	third := resolve(video, "video")
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.Equal(t, "quota_exceeded", tiktokToolCode(third))
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
