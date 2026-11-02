package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorOrdinaryProbeBoundsUpstreamRequestAndBody(t *testing.T) {
	for _, capability := range []struct{ model, path, mapping, input string }{
		{"x-user", "/api/v1/twitter/web/fetch_user_profile", "screen_name", `"screen_name":"fixture"`},
		{"x-posts", "/api/v1/twitter/web/fetch_user_post_tweet", "screen_name", `"screen_name":"fixture"`},
		{"tiktok-user", "/api/v1/tiktok/web/fetch_user_profile", "uniqueId", `"uniqueId":"fixture"`},
		{"tiktok-posts", "/api/v1/tiktok/app/v3/fetch_user_post_videos", "unique_id", `"unique_id":"fixture"`},
	} {
		for _, phase := range []string{"before-headers", "during-body", "oversized-body"} {
			t.Run(capability.model+"/"+phase, func(t *testing.T) {
				db := setupModelListControllerTestDB(t)
				require.NoError(t, db.AutoMigrate(&model.Log{}))
				prices := ratio_setting.ModelPrice2JSONString()
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"x-user":0.001,"x-posts":0.001,"tiktok-user":0.001,"tiktok-posts":0.001}`))
				t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices)) })
				user := model.User{Username: "bounded-probe-admin", Group: "default", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 100000}
				require.NoError(t, db.Create(&user).Error)
				entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
				releaseUpstream := sync.OnceFunc(func() { close(release) })
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, capability.mapping+"=fixture", r.URL.RawQuery)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Upstream-Secret", "private-header-sentinel")
					if phase == "during-body" {
						_, _ = io.WriteString(w, `{`)
						w.(http.Flusher).Flush()
					} else if phase == "oversized-body" {
						body := `{"data":{"user":{"nickname":"private-body-sentinel"},"items":[]}}` + strings.Repeat(" ", 2*1024*1024)
						_, _ = io.WriteString(w, body)
						w.(http.Flusher).Flush()
					}
					close(entered)
					select {
					case <-r.Context().Done():
						close(canceled)
					case <-release:
						if phase == "before-headers" {
							_, _ = io.WriteString(w, `{"data":{"user":{"nickname":"fixture"},"items":[]}}`)
						} else if phase == "during-body" {
							_, _ = io.WriteString(w, `"data":{"user":{"nickname":"fixture"},"items":[]}}`)
						}
					}
				}))
				defer func() {
					releaseUpstream()
					server.CloseClientConnections()
					server.Close()
				}()
				channel := model.Channel{Type: constant.ChannelTypeAdvancedCustom, Key: "private-key-sentinel", Name: "Stopped bounded fixture", Status: common.ChannelStatusManuallyDisabled, Models: capability.model, Group: "default", BaseURL: common.GetPointer(server.URL)}
				channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{IncomingPath: "/v1/chat/completions", UpstreamPath: capability.path, Models: []string{capability.model}, UpstreamMethod: "GET", QueryMapping: map[string]string{capability.mapping: capability.mapping}}}}})
				require.NoError(t, db.Create(&channel).Error)
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Params = gin.Params{{Key: "id", Value: common.Interface2String(channel.Id)}}
				c.Set("id", user.Id)
				ctx, cancel := context.WithCancel(context.Background())
				if phase != "oversized-body" {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 300*time.Millisecond)
				}
				defer cancel()
				c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/channel/test/1", strings.NewReader(`{"model":"`+capability.model+`",`+capability.input+`}`))
				c.Request.Header.Set("Content-Type", "application/json")
				done := make(chan struct{})
				go func() { defer close(done); TestAccountMonitorChannel(c) }()
				select {
				case <-entered:
				case <-time.After(time.Second):
					releaseUpstream()
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Error("management fixture did not stop after local upstream release")
					}
					t.Fatal("local upstream was not reached")
				}
				returned := false
				select {
				case <-done:
					returned = true
				case <-time.After(400 * time.Millisecond):
					t.Error("bounded management handler did not return")
				}
				select {
				case <-canceled:
				case <-time.After(200 * time.Millisecond):
					t.Error("local upstream did not observe request context cancellation or body close")
				}
				releaseUpstream()
				if !returned {
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Fatal("management fixture did not stop after local upstream release")
					}
				}
				var response map[string]any
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(t, false, response["success"])
				assert.Equal(t, "other_failure", response["failure_kind"])
				assert.NotContains(t, response, "shape_evidence")
				assert.NotContains(t, response, "data")
				assert.NotContains(t, recorder.Body.String(), "sentinel")
				assert.NotContains(t, recorder.Body.String(), "fixture")
				assert.Empty(t, recorder.Header().Get("X-Upstream-Secret"))
				var logs int64
				require.NoError(t, db.Model(&model.Log{}).Where("channel_id = ?", channel.Id).Count(&logs).Error)
				assert.Zero(t, logs, "canceled/oversized probes must not settle successful usage")
			})
		}
	}
}

func TestAccountMonitorProbeRejectsUntrustedInput(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"model":"gpt-4o","screen_name":"public"}`,
		`{"model":"x-user","screen_name":"public","url":"https://example.com"}`,
		`{"model":"x-user","screen_name":"public","headers":{}}`,
		`{"model":"x-user","screen_name":"public","key":"secret"}`,
		`{"model":"x-user","screen_name":"public","body":{}}`,
		`{"model":"x-user","screen_name":"public","account_monitor_probe":true}`,
		`{"model":"x-user","screen_name":"public","bounded_probe":true}`,
		`{"model":"x-user","screen_name":42}`, `{"model":"x-user","screen_name":null}`,
		`{"model":"x-user","screen_name":"https://x.com/public"}`,
		`{"model":"x-user","screen_name":"public","cursor":"next"}`,
		`{"model":"x-posts","screen_name":"public","cursor":{}}`,
		`{"model":"x-user","screen_name":"public","rest_id":"123"}`,
		`{"model":"tiktok-posts","unique_id":"public","count":0}`,
		`{"model":"tiktok-posts","unique_id":"public","count":51}`,
		`{"model":"tiktok-posts","unique_id":"public","count":"20"}`,
		`{"model":"tiktok-posts","unique_id":"public","sort_type":1}`,
		`{"model":"tiktok-posts","unique_id":"public","region":"US"}`,
		`{"model":"tiktok-posts","unique_id":"public","max_cursor":-1}`,
		`{"model":"tiktok-user","uniqueId":"` + strings.Repeat("x", 65) + `"}`,
		`{"model":"x-posts","screen_name":"public","cursor":"` + strings.Repeat("x", 513) + `"}`,
		`{"model":"x-user","screen_name":"public"} {}`,
	} {
		t.Run(body[:min(len(body), 90)], func(t *testing.T) {
			_, err := parseAccountMonitorProbe(strings.NewReader(body))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
	_, err := parseAccountMonitorProbe(strings.NewReader(strings.Repeat(" ", 4097)))
	require.Error(t, err)
}

func TestAccountMonitorProbeSupportsOnlyPublicIdentifiersAndPaging(t *testing.T) {
	for _, body := range []string{
		`{"model":"x-user","screen_name":"higgsfield_ai"}`,
		`{"model":"x-posts","rest_id":"12345","cursor":"ABC/+=="}`,
		`{"model":"tiktok-user","uniqueId":"gethullo"}`,
		`{"model":"tiktok-user","secUid":"MS4wLjABAAAA-test"}`,
		`{"model":"tiktok-posts","unique_id":"gethullo","count":20,"max_cursor":0}`,
	} {
		_, err := parseAccountMonitorProbe(strings.NewReader(body))
		require.NoError(t, err, body)
	}
}

func TestAccountMonitorProbeRejectsUnboundedSavedRoutes(t *testing.T) {
	probe, err := parseAccountMonitorProbe(strings.NewReader(`{"model":"x-user","screen_name":"public"}`))
	require.NoError(t, err)
	for _, scenario := range []struct {
		name   string
		modify func(*model.Channel, *dto.AdvancedCustomRoute)
	}{
		{name: "catch-all model", modify: func(_ *model.Channel, r *dto.AdvancedCustomRoute) { r.Models = nil }},
		{name: "other capability", modify: func(_ *model.Channel, r *dto.AdvancedCustomRoute) { r.Models = []string{"x-posts"} }},
		{name: "arbitrary upstream", modify: func(_ *model.Channel, r *dto.AdvancedCustomRoute) { r.UpstreamPath = "https://untrusted.example" }},
		{name: "write method", modify: func(_ *model.Channel, r *dto.AdvancedCustomRoute) { r.UpstreamMethod = "POST" }},
		{name: "nested input mapping", modify: func(_ *model.Channel, r *dto.AdvancedCustomRoute) {
			r.QueryMapping = map[string]string{"screen_name": "raw.screen_name"}
		}},
		{name: "unrelated channel type", modify: func(c *model.Channel, _ *dto.AdvancedCustomRoute) { c.Type = constant.ChannelTypeOpenAI }},
		{name: "malformed transport settings", modify: func(c *model.Channel, _ *dto.AdvancedCustomRoute) { c.Setting = common.GetPointer("invalid-json") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			channel := &model.Channel{Type: constant.ChannelTypeAdvancedCustom, Models: "x-user", Key: "saved-secret"}
			route := dto.AdvancedCustomRoute{IncomingPath: "/v1/chat/completions", UpstreamPath: "/api/v1/twitter/web/fetch_user_profile", Models: []string{"x-user"}, UpstreamMethod: "GET", QueryMapping: map[string]string{"screen_name": "screen_name"}}
			scenario.modify(channel, &route)
			channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{route}}})
			before, err := common.Marshal(channel)
			require.NoError(t, err)
			_, err = accountMonitorProbeChannel(channel, probe)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "saved-secret")
			after, err := common.Marshal(channel)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
		})
	}
}

func TestAccountMonitorProbeUsesFormalAdapterWithoutChangingSavedChannel(t *testing.T) {
	for _, scenario := range []struct {
		name               string
		model              string
		input              string
		body               string
		query              string
		wantSuccess        bool
		wantCount          int
		wantFailureKind    string
		wantUpstreamStatus int
		status             int
		wantShapeEvidence  bool
		shapeHeader        bool
	}{
		{name: "profile", model: "x-user", input: `"screen_name":"higgsfield_ai"`, body: `{"code":200,"data":{"user":{"rest_id":"123","name":"Higgsfield","follower_count":0}}}`, query: "screen_name=higgsfield_ai", wantSuccess: true},
		{name: "empty posts", model: "tiktok-posts", input: `"unique_id":"gethullo","count":20,"max_cursor":0`, body: `{"code":200,"data":{"aweme_list":[]}}`, query: "count=20&max_cursor=0&unique_id=gethullo", wantSuccess: true},
		{name: "posts summary", model: "tiktok-posts", input: `"sec_user_id":"MS4wLjAB-test","count":1`, body: `{"code":200,"data":{"aweme_list":[{"aweme_id":"123","video":{"play_addr":{"url_list":["https://media.example/signed?secret=value"]}}}]}}`, query: "count=1&sec_user_id=MS4wLjAB-test", wantSuccess: true, wantCount: 1},
		{name: "X direct timeline existing rows", model: "x-posts", input: `"screen_name":"higgsfield_ai","shape_diagnostic":true`, body: `{"data":{"timeline":[{"id":"fixture-parent","node":{"id":"fixture-child"}},{"node":{"tweet_id":"fixture-node"}},null,{}]}}`, query: "screen_name=higgsfield_ai", wantSuccess: true, wantCount: 2},
		{name: "X direct timeline bad rows ordinary", model: "x-posts", input: `"screen_name":"higgsfield_ai"`, body: `{"data":{"timeline":[{"id":{}},null]}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "posts_unrecognized", wantUpstreamStatus: http.StatusBadGateway},
		{name: "X direct timeline bad rows typed shape", model: "x-posts", input: `"screen_name":"higgsfield_ai","shape_diagnostic":true`, body: `{"data":{"timeline":[{"id":{}},null]}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "posts_unrecognized", wantUpstreamStatus: http.StatusBadGateway, wantShapeEvidence: true},
		{name: "X timeline user business failure ordinary", model: "x-posts", input: `"screen_name":"higgsfield_ai"`, body: `{"data":{"user":{"success":false},"timeline":[{"id":"secret-rejected-post"}]}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "X timeline author business failure without shape", model: "x-posts", input: `"screen_name":"higgsfield_ai","shape_diagnostic":true`, body: `{"data":{"author":{"ok":false},"timeline":[{"id":"secret-rejected-post"}]}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "business failure", model: "x-user", input: `"screen_name":"higgsfield_ai"`, body: `{"code":401,"message":"secret-diagnostic-value","data":{}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "unknown profile", model: "x-user", input: `"screen_name":"higgsfield_ai"`, body: `{"code":200,"data":{"unexpected":"secret-profile-response"}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "profile_unrecognized", wantUpstreamStatus: http.StatusBadGateway},
		{name: "invalid JSON", model: "x-user", input: `"screen_name":"higgsfield_ai"`, body: `<html>secret-response</html>`, query: "screen_name=higgsfield_ai", wantFailureKind: "invalid_response", wantUpstreamStatus: http.StatusBadGateway},
		{name: "unknown posts", model: "tiktok-posts", input: `"unique_id":"gethullo"`, body: `{"code":200,"data":{"unexpected":[]}}`, query: "unique_id=gethullo", wantFailureKind: "posts_unrecognized", wantUpstreamStatus: http.StatusBadGateway},
		{name: "HTTP failure", model: "x-user", input: `"screen_name":"higgsfield_ai"`, body: `{"unrecognized":"secret-http-response"}`, query: "screen_name=higgsfield_ai", status: http.StatusUnauthorized, wantFailureKind: "upstream_http_error", wantUpstreamStatus: http.StatusUnauthorized},
		{name: "HTTP failure without response payload", model: "tiktok-posts", input: `"unique_id":"gethullo"`, query: "unique_id=gethullo", status: http.StatusUnauthorized, wantFailureKind: "upstream_http_error", wantUpstreamStatus: http.StatusUnauthorized},
		{name: "HTTP gateway failure", model: "x-user", input: `"screen_name":"higgsfield_ai"`, body: `{"unrecognized":"secret-http-response"}`, query: "screen_name=higgsfield_ai", status: http.StatusBadGateway, wantFailureKind: "upstream_http_error", wantUpstreamStatus: http.StatusBadGateway},
		{name: "shape disabled by omission", model: "tiktok-user", input: `"uniqueId":"gethullo"`, body: `{"code":200,"data":{"userInfo":{"user":{"nickname":"secret-profile"}},"secret-unknown-key":"secret-unknown-value"}}`, query: "uniqueId=gethullo", wantFailureKind: "profile_unrecognized", wantUpstreamStatus: http.StatusBadGateway},
		{name: "shape disabled by false", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":false`, body: `{"code":200,"data":{"userInfo":{"user":{"nickname":"secret-profile"}}}}`, query: "uniqueId=gethullo", wantFailureKind: "profile_unrecognized", wantUpstreamStatus: http.StatusBadGateway},
		{name: "shape header cannot enable", model: "tiktok-user", input: `"uniqueId":"gethullo"`, body: `{"code":200,"data":{"userInfo":{"user":{"nickname":"secret-profile"}}}}`, query: "uniqueId=gethullo", wantFailureKind: "profile_unrecognized", wantUpstreamStatus: http.StatusBadGateway, shapeHeader: true},
		{name: "profile typed shape", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"code":200,"data":{"userInfo":{"user":{"nickname":" ","signature":"\t","avatarLarger":"secret-invalid-avatar"},"stats":{"followerCount":-987654321,"videoCount":-1}},"secret-unknown-key":"secret-unknown-value"}}`, query: "uniqueId=gethullo", wantFailureKind: "profile_unrecognized", wantUpstreamStatus: http.StatusBadGateway, wantShapeEvidence: true},
		{name: "posts typed shape", model: "x-posts", input: `"screen_name":"higgsfield_ai","shape_diagnostic":true`, body: `{"code":200,"data":{"timeline":{"instructions":[{"entries":[{"content":{"itemContent":{"tweet_results":{"result":{"rest_id":"secret-post-id","legacy":{"full_text":"secret-post-text","created_at":"secret-created-at","entities":{"media":["secret-media"]}}}}}}}]}]},"secret-unknown-key":"secret-unknown-value"}}`, query: "screen_name=higgsfield_ai", wantFailureKind: "posts_unrecognized", wantUpstreamStatus: http.StatusBadGateway, wantShapeEvidence: true},
		{name: "observed TikTok profile", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"code":200,"data":{"userInfo":{"user":{"nickname":"Fixture Creator","signature":"Fixture biography","avatarLarger":"https://example.com/avatar.jpg"},"stats":{"followerCount":0,"videoCount":2}}}}`, query: "uniqueId=gethullo", wantSuccess: true},
		{name: "observed wrapper failure", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"data":{"nickname":"secret-fallback","userInfo":{"success":false,"user":{"nickname":"secret-profile"},"stats":{}}}}`, query: "uniqueId=gethullo", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "observed user failure", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"data":{"nickname":"secret-fallback","userInfo":{"user":{"ok":false,"nickname":"secret-profile"},"stats":{}}}}`, query: "uniqueId=gethullo", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "observed stats failure", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"data":{"nickname":"secret-fallback","userInfo":{"user":{"nickname":"secret-profile"},"stats":{"code":500,"followerCount":0}}}}`, query: "uniqueId=gethullo", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "shape success stays summary only", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"code":200,"data":{"user":{"nickname":"secret-valid-profile","follower_count":0}}}`, query: "uniqueId=gethullo", wantSuccess: true},
		{name: "shape business failure stays classified", model: "tiktok-user", input: `"uniqueId":"gethullo","shape_diagnostic":true`, body: `{"code":401,"data":{"userInfo":{"user":{"nickname":"secret-profile"}}}}`, query: "uniqueId=gethullo", wantFailureKind: "upstream_business_failure", wantUpstreamStatus: http.StatusBadGateway},
		{name: "shape invalid JSON stays classified", model: "x-posts", input: `"screen_name":"higgsfield_ai","shape_diagnostic":true`, body: `<html>secret-response</html>`, query: "screen_name=higgsfield_ai", wantFailureKind: "invalid_response", wantUpstreamStatus: http.StatusBadGateway},
		{name: "shape HTTP failure stays classified", model: "x-posts", input: `"screen_name":"higgsfield_ai","shape_diagnostic":true`, body: `{"code":200,"data":{"timeline":{"instructions":[]}}}`, query: "screen_name=higgsfield_ai", status: http.StatusForbidden, wantFailureKind: "upstream_http_error", wantUpstreamStatus: http.StatusForbidden},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			require.NoError(t, db.AutoMigrate(&model.Log{}))
			prices := ratio_setting.ModelPrice2JSONString()
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"x-user":0.001,"x-posts":0.001,"tiktok-user":0.001,"tiktok-posts":0.001}`))
			t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices)) })
			user := model.User{Username: "probe-admin", Group: "default", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 100000}
			require.NoError(t, db.Create(&user).Error)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, scenario.query, r.URL.RawQuery)
				assert.NotContains(t, r.URL.RawQuery, "shape_diagnostic")
				assert.Empty(t, r.Header.Get("shape_diagnostic"))
				requestBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Empty(t, requestBody)
				assert.Equal(t, "Bearer secret-saved-key", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Upstream-Diagnostic", "secret-header-value")
				if scenario.status != 0 {
					w.WriteHeader(scenario.status)
				}
				_, _ = w.Write([]byte(scenario.body))
			}))
			defer server.Close()
			path := "/api/v1/twitter/web/fetch_user_profile"
			mapping := map[string]string{"screen_name": "screen_name", "rest_id": "rest_id"}
			if scenario.model == "x-posts" {
				path = "/api/v1/twitter/web/fetch_user_post_tweet"
				mapping["cursor"] = "cursor"
			}
			if scenario.model == "tiktok-user" {
				path = "/api/v1/tiktok/web/fetch_user_profile"
				mapping = map[string]string{"uniqueId": "uniqueId", "secUid": "secUid"}
			}
			if scenario.model == "tiktok-posts" {
				path = "/api/v1/tiktok/app/v3/fetch_user_post_videos"
				// The stopped production source also retains these legacy fields.
				mapping = map[string]string{"unique_id": "unique_id", "sec_user_id": "sec_user_id", "count": "count", "max_cursor": "max_cursor", "sort_type": "sort_type", "region": "region"}
			}
			channel := model.Channel{Type: constant.ChannelTypeAdvancedCustom, Key: "secret-saved-key", Name: "Stopped source", Status: common.ChannelStatusManuallyDisabled, Models: scenario.model, Group: "default", BaseURL: common.GetPointer(server.URL)}
			if scenario.name == "profile" {
				channel.Key += "\nsecret-disabled-key"
				channel.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling, MultiKeyPollingIndex: 1, MultiKeyStatusList: map[int]int{0: common.ChannelStatusEnabled, 1: common.ChannelStatusManuallyDisabled}}
			}
			channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{IncomingPath: "/v1/chat/completions", UpstreamPath: path, Models: []string{scenario.model}, UpstreamMethod: "GET", QueryMapping: mapping}}}})
			require.NoError(t, db.Create(&channel).Error)
			before, err := common.Marshal(channel)
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: common.Interface2String(channel.Id)}}
			c.Set("id", user.Id)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/test/1", strings.NewReader(`{"model":"`+scenario.model+`",`+scenario.input+`}`))
			c.Request.Header.Set("Content-Type", "application/json")
			if scenario.shapeHeader {
				c.Request.Header.Set("shape_diagnostic", "true")
			}
			TestAccountMonitorChannel(c)
			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success        bool   `json:"success"`
				FailureKind    string `json:"failure_kind"`
				ErrorCode      string `json:"error_code"`
				Message        string `json:"message"`
				UpstreamStatus *int   `json:"upstream_status"`
				Data           struct {
					ProfileRecognized bool `json:"profile_recognized"`
					PostCount         int  `json:"post_count"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, scenario.wantSuccess, response.Success, recorder.Body.String())
			assert.Equal(t, scenario.wantFailureKind, response.FailureKind)
			var responseFields map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &responseFields))
			shapeEvidence, hasShapeEvidence := responseFields["shape_evidence"]
			assert.Equal(t, scenario.wantShapeEvidence, hasShapeEvidence)
			if scenario.wantShapeEvidence {
				evidence, ok := shapeEvidence.(map[string]any)
				require.True(t, ok)
				version, slots := 1, 22
				if scenario.model == "x-posts" {
					version, slots = 3, 32
				}
				assert.EqualValues(t, version, evidence["schema_version"])
				assert.Len(t, evidence["slots"], slots)
				assert.Equal(t, scenario.model, evidence["model"])
				assert.NotEmpty(t, evidence["slots"])
			}
			if scenario.wantSuccess {
				assert.NotContains(t, recorder.Body.String(), "failure_kind")
				assert.Nil(t, response.UpstreamStatus)
				assert.Empty(t, response.ErrorCode)
			} else {
				assert.Equal(t, "account_monitor_channel_test_failed", response.ErrorCode)
				assert.Equal(t, "account monitor channel test failed", response.Message)
				require.NotNil(t, response.UpstreamStatus)
				assert.Equal(t, scenario.wantUpstreamStatus, *response.UpstreamStatus)
			}
			assert.Equal(t, scenario.wantCount, response.Data.PostCount)
			assert.Equal(t, scenario.wantSuccess && (scenario.model == "x-user" || scenario.model == "tiktok-user"), response.Data.ProfileRecognized)
			assert.NotContains(t, recorder.Body.String(), "secret")
			assert.NotContains(t, recorder.Body.String(), "aweme_list")
			assert.NotContains(t, recorder.Body.String(), "media.example")
			assert.NotContains(t, recorder.Body.String(), "higgsfield_ai")
			assert.NotContains(t, recorder.Body.String(), "gethullo")
			assert.NotContains(t, recorder.Body.String(), "rest_id")
			assert.NotContains(t, recorder.Body.String(), "987654321")
			assert.Empty(t, recorder.Header().Get("X-Upstream-Diagnostic"))
			assert.Equal(t, 1, calls)
			var logCount int64
			require.NoError(t, db.Model(&model.Log{}).Where("channel_id = ?", channel.Id).Count(&logCount).Error)
			if scenario.wantSuccess {
				assert.EqualValues(t, 1, logCount)
			} else {
				assert.Zero(t, logCount)
			}
			var saved model.Channel
			require.NoError(t, db.First(&saved, channel.Id).Error)
			after, err := common.Marshal(saved)
			require.NoError(t, err)
			assert.JSONEq(t, string(before), string(after))
		})
	}
}

func TestAccountMonitorProbeFailureKindRejectsUntrustedErrors(t *testing.T) {
	assert.Equal(t, "other_failure", accountMonitorProbeFailureKind(nil))
	for _, scenario := range []struct {
		name    string
		code    types.ErrorCode
		message string
	}{
		{"dynamic upstream detail", types.ErrorCodeBadResponseBody, "Bearer secret-saved-key https://provider.example/private?account_id=123"},
		{"known prefix with dynamic suffix", types.ErrorCodeBadResponseBody, "account monitoring upstream profile is unrecognized: secret-saved-key"},
		{"known text wrapped in raw body", types.ErrorCodeBadResponseBody, `{"message":"account monitoring upstream reported a failure","token":"secret-saved-key"}`},
		{"known body message with wrong code", types.ErrorCodeBadResponseStatusCode, "account monitoring upstream reported a failure"},
		{"known HTTP message with wrong code", types.ErrorCodeBadResponseBody, "account monitor upstream returned an HTTP error"},
		{"unknown code", types.ErrorCode("upstream-secret-code"), "account monitoring upstream posts are unrecognized"},
		{"known message with whitespace", types.ErrorCodeBadResponseBody, " account monitoring upstream response is invalid"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			apiErr := types.NewOpenAIError(errors.New(scenario.message), scenario.code, http.StatusBadGateway)
			kind := accountMonitorProbeFailureKind(apiErr)
			assert.Equal(t, "other_failure", kind)
			assert.NotContains(t, kind, "secret")
			assert.NotContains(t, kind, "provider.example")
		})
	}
}
