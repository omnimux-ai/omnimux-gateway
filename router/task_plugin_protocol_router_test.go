package router

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostProtocolRegistryDrivesProtocolRoutesOnce(t *testing.T) {
	engine := gin.New()
	SetTaskPluginProtocolRouter(engine)

	expected := []string{
		"POST /v1/responses",
		"GET /v1/responses/:response_id",
		"POST /v1/videos",
		"GET /v1/videos/:task_id",
		"GET /v1/videos/:task_id/content",
		"HEAD /v1/videos/:task_id/content",
		"POST /v1/images/generations",
		"POST /v1/images/edits",
		"POST /v1/edits",
	}
	actual := make([]string, 0, len(engine.Routes()))
	for _, route := range engine.Routes() {
		actual = append(actual, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}

func TestImageProtocolRouterPreservesGroupRoutingAndNativeBilling(t *testing.T) {
	for _, tc := range []struct {
		name, path, requestModel, header, legacyHeader, wantGroup string
		wantStatus, wantCalls, wantCharge                         int
	}{
		{name: "public model suffix selects cheap", path: "/v1/images/generations", requestModel: "qwen-image@cheap", wantGroup: "cheap", wantStatus: 200, wantCalls: 1, wantCharge: 25000},
		{name: "header overrides suffix", path: "/v1/images/generations", requestModel: "qwen-image@cheap", header: "default", wantGroup: "default", wantStatus: 200, wantCalls: 1, wantCharge: 50000},
		{name: "new header overrides legacy header", path: "/v1/images/generations", requestModel: "qwen-image@cheap", header: "default", legacyHeader: "cheap", wantGroup: "default", wantStatus: 200, wantCalls: 1, wantCharge: 50000},
		{name: "legacy header overrides suffix", path: "/v1/images/generations", requestModel: "qwen-image@cheap", legacyHeader: "default", wantGroup: "default", wantStatus: 200, wantCalls: 1, wantCharge: 50000},
		{name: "suffix whitespace follows distributor syntax", path: "/v1/images/generations", requestModel: "qwen-image @ cheap ", wantGroup: "cheap", wantStatus: 200, wantCalls: 1, wantCharge: 25000},
		{name: "forbidden suffix rejected before upstream", path: "/v1/images/generations", requestModel: "qwen-image@forbidden", wantStatus: 403},
		{name: "forbidden header overrides allowed suffix", path: "/v1/images/generations", requestModel: "qwen-image@cheap", header: "forbidden", wantStatus: 403},
		{name: "canonical edit uses native image protocol", path: "/v1/images/edits", requestModel: "qwen-image-edit", wantGroup: "default", wantStatus: 200, wantCalls: 1, wantCharge: 50000},
		{name: "compatibility edit uses same native image protocol", path: "/v1/edits", requestModel: "qwen-image-edit", wantGroup: "default", wantStatus: 200, wantCalls: 1, wantCharge: 50000},
		{name: "compatibility edit keeps group suffix", path: "/v1/edits", requestModel: "qwen-image-edit@cheap", wantGroup: "cheap", wantStatus: 200, wantCalls: 1, wantCharge: 25000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupImageProtocolRoutingTest(t)
			calls := 0
			var received map[string]any
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, "Bearer "+tc.wantGroup+"-upstream", r.Header.Get("Authorization"))
				assert.Equal(t, "/api/v1/services/aigc/multimodal-generation/generation", r.URL.Path)
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, common.Unmarshal(raw, &received))
				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write([]byte(`{"request_id":"image-local","output":{"choices":[{"message":{"content":[{"image":"https://cdn.example/image.png"}]}}]},"usage":{"image_count":1}}`))
				require.NoError(t, err)
			}))
			t.Cleanup(upstream.Close)
			for id, group := range []string{"default", "cheap"} {
				channel := model.Channel{Id: 100 + id, Type: constant.ChannelTypeAli, Status: common.ChannelStatusEnabled, Key: group + "-upstream", Name: group, Models: "qwen-image,qwen-image-edit", Group: group, BaseURL: &upstream.URL}
				require.NoError(t, model.DB.Create(&channel).Error)
				require.NoError(t, channel.UpdateAbilities(nil))
			}
			model.InitChannelCache()
			engine := gin.New()
			SetRelayRouter(engine)
			SetTaskPluginProtocolRouter(engine)
			body := map[string]any{"model": tc.requestModel, "prompt": "watercolor", "n": 1, "seed": 0, "watermark": false, "size": "1024x1024", "prompt_extend": false}
			if strings.HasSuffix(tc.path, "/edits") {
				body["images"] = []string{"https://cdn.example/input.png"}
			}
			raw, err := common.Marshal(body)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(raw)))
			request.Header.Set("Authorization", "Bearer imageroutingkey")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Omnimux-Group", tc.header)
			request.Header.Set("New-Api-Group", tc.legacyHeader)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.wantCalls, calls)
			var user model.User
			require.NoError(t, model.DB.First(&user, 90).Error)
			assert.Equal(t, 10000000-tc.wantCharge, user.Quota)
			if tc.wantCalls == 0 {
				return
			}
			require.NotNil(t, received)
			wantModel := "qwen-image"
			if strings.HasSuffix(tc.path, "/edits") {
				wantModel = "qwen-image-edit"
			}
			assert.Equal(t, wantModel, received["model"])
			parameters := received["parameters"].(map[string]any)
			assert.Equal(t, float64(0), parameters["seed"])
			assert.Equal(t, false, parameters["watermark"])
			assert.Equal(t, false, parameters["prompt_extend"])
			assert.Equal(t, float64(1), parameters["n"])
			assert.Contains(t, recorder.Body.String(), "https://cdn.example/image.png")
			var task model.Task
			require.NoError(t, model.DB.First(&task).Error)
			assert.Equal(t, tc.wantGroup, task.Group)
			assert.Equal(t, wantModel, task.Properties.OriginModelName)
			assert.Equal(t, tc.wantCharge, task.Quota)
		})
	}
}

func TestImageProtocolRouterKeepsUnclaimedChannelFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name, path, publicModel, mappedModel, wantUpstreamPath, response string
		channelType                                                      int
		async                                                            bool
	}{
		{name: "ordinary unknown model generation", path: "/v1/images/generations", publicModel: "ordinary-image", channelType: constant.ChannelTypeOpenAI, wantUpstreamPath: "/v1/images/generations", response: `{"created":1710000000,"data":[{"url":"https://cdn.example/ordinary.png"}]}`},
		{name: "ordinary unknown model edit alias", path: "/v1/edits", publicModel: "ordinary-image", channelType: constant.ChannelTypeOpenAI, wantUpstreamPath: "/v1/images/edits", response: `{"created":1710000000,"data":[{"url":"https://cdn.example/ordinary.png"}]}`},
		{name: "GoEasy asynchronous generation", path: "/v1/images/generations", publicModel: "gpt-image-2.5", mappedModel: "gpt-image-2.5-async", channelType: constant.ChannelTypeGoEasyAPI, wantUpstreamPath: "/api/image-tasks/generations", response: `{"id":"upstream-pending","status":"processing"}`, async: true},
		{name: "GoEasy asynchronous edit alias", path: "/v1/edits", publicModel: "gpt-image-2.5", mappedModel: "gpt-image-2.5-async", channelType: constant.ChannelTypeGoEasyAPI, wantUpstreamPath: "/api/image-tasks/edits", response: `{"id":"upstream-pending","status":"processing"}`, async: true},
		{name: "ordinary mapped vendor model remains asynchronous", path: "/v1/images/generations", publicModel: "qwen-image-3-0", mappedModel: "qwen-image-3.0", channelType: constant.ChannelTypeOpenAI, wantUpstreamPath: "/v1/images/generations", response: `{"id":"upstream-pending","status":"queued"}`, async: true},
		{name: "ordinary mapped vendor edit alias remains asynchronous", path: "/v1/edits", publicModel: "qwen-image-3-0", mappedModel: "qwen-image-3.0", channelType: constant.ChannelTypeOpenAI, wantUpstreamPath: "/v1/images/edits", response: `{"id":"upstream-pending","status":"queued"}`, async: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupImageProtocolRoutingTest(t)
			calls := 0
			var received map[string]any
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, tc.wantUpstreamPath, r.URL.Path)
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, common.Unmarshal(raw, &received))
				w.Header().Set("Content-Type", "application/json")
				_, err = w.Write([]byte(tc.response))
				require.NoError(t, err)
			}))
			t.Cleanup(upstream.Close)
			channel := model.Channel{Id: 37, Type: tc.channelType, Key: "fallback-upstream", Status: common.ChannelStatusEnabled, Name: "ordinary fallback", Models: tc.publicModel, Group: "default", BaseURL: &upstream.URL}
			if tc.mappedModel != "" && tc.publicModel == "qwen-image-3-0" {
				mapping, err := common.Marshal(map[string]string{tc.publicModel: tc.mappedModel})
				require.NoError(t, err)
				channel.ModelMapping = common.GetPointer(string(mapping))
			}
			require.NoError(t, model.DB.Create(&channel).Error)
			require.NoError(t, channel.UpdateAbilities(nil))
			model.InitChannelCache()
			engine := gin.New()
			SetRelayRouter(engine)
			SetTaskPluginProtocolRouter(engine)
			body := map[string]any{"model": tc.publicModel, "prompt": "watercolor", "n": 1, "size": "1024x1024"}
			if strings.HasSuffix(tc.path, "/edits") {
				body["images"] = []string{"https://cdn.example/input.png"}
			}
			raw, err := common.Marshal(body)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(raw)))
			request.Header.Set("Authorization", "Bearer imageroutingkey")
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, 1, calls)
			wantModel := tc.publicModel
			if tc.mappedModel != "" {
				wantModel = tc.mappedModel
			}
			assert.Equal(t, wantModel, received["model"])
			assert.Equal(t, "watercolor", received["prompt"])
			if tc.async {
				assert.NotContains(t, recorder.Body.String(), "upstream-pending")
				var task model.Task
				require.NoError(t, model.DB.First(&task).Error)
				assert.Equal(t, tc.publicModel, task.Properties.OriginModelName)
				wantPlatform := constant.TaskPlatform(fmt.Sprint(tc.channelType))
				if tc.channelType == constant.ChannelTypeOpenAI {
					wantPlatform = constant.TaskPlatform("55")
				}
				assert.Equal(t, wantPlatform, task.Platform)
				assert.Equal(t, 50000, task.Quota)
				assert.Contains(t, recorder.Body.String(), task.TaskID)
			} else {
				assert.Contains(t, recorder.Body.String(), "https://cdn.example/ordinary.png")
			}
			var user model.User
			require.NoError(t, model.DB.First(&user, 90).Error)
			assert.Equal(t, 9950000, user.Quota)
		})
	}
}

func TestImageProtocolRouterKeepsOrdinaryAndPluginGroupsCoexisting(t *testing.T) {
	for _, publicModel := range []string{"qwen-image-3-0", "qwen-image-3.0"} {
		for _, tc := range []struct {
			name, suffix, bodyGroup, header, legacyHeader, tokenGroup, autoGroups, authorization, wantGroup string
			wantStatus                                                                                      int
			plugin, memoryCache                                                                             bool
		}{
			{name: "default native", wantGroup: "default", wantStatus: 200},
			{name: "body cheap plugin", bodyGroup: "cheap", wantGroup: "cheap", wantStatus: 200, plugin: true},
			{name: "suffix cheap plugin", suffix: "@cheap", wantGroup: "cheap", wantStatus: 200, plugin: true},
			{name: "header native overrides suffix and body", suffix: "@cheap", bodyGroup: "cheap", header: "default", legacyHeader: "cheap", wantGroup: "default", wantStatus: 200},
			{name: "legacy header native overrides suffix", suffix: "@cheap", legacyHeader: "default", wantGroup: "default", wantStatus: 200},
			{name: "header plugin overrides native suffix", suffix: "@default", header: "cheap", wantGroup: "cheap", wantStatus: 200, plugin: true},
			{name: "forbidden header", suffix: "@cheap", header: "forbidden", wantStatus: 403},
			{name: "forbidden body", bodyGroup: "forbidden", wantStatus: 403},
			{name: "token cheap", tokenGroup: "cheap", wantGroup: "cheap", wantStatus: 200, plugin: true},
			{name: "token auto native first", tokenGroup: "auto", autoGroups: `["default","cheap"]`, wantGroup: "default", wantStatus: 200},
			{name: "token auto plugin first", tokenGroup: "auto", autoGroups: `["cheap","default"]`, wantGroup: "cheap", wantStatus: 200, plugin: true},
			{name: "admin native pin overrides cheap", header: "cheap", authorization: "imageroutingkey-37", wantGroup: "cheap", wantStatus: 200},
			{name: "admin plugin pin overrides default", authorization: "imageroutingkey-17", wantGroup: "default", wantStatus: 200, plugin: true},
			{name: "memory cache native first", tokenGroup: "auto", autoGroups: `["default","cheap"]`, wantGroup: "default", wantStatus: 200, memoryCache: true},
			{name: "memory cache plugin first", tokenGroup: "auto", autoGroups: `["cheap","default"]`, wantGroup: "cheap", wantStatus: 200, plugin: true, memoryCache: true},
		} {
			t.Run(publicModel+"/"+tc.name, func(t *testing.T) {
				setupImageProtocolRoutingTest(t)
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"qwen-image-3-0":0.1,"qwen-image-3.0":0.1}`))
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","cheap":"Cheap","auto":"Auto"}`))
				if tc.tokenGroup != "" {
					require.NoError(t, model.DB.Model(&model.Token{}).Where("key = ?", "imageroutingkey").Updates(map[string]any{"group": tc.tokenGroup, "auto_groups": tc.autoGroups}).Error)
				}
				if tc.authorization != "" {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 90).Update("role", common.RoleAdminUser).Error)
				}
				calls := 0
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					wantPath, wantKey := "/v1/images/generations", "ordinary-upstream"
					if tc.plugin {
						wantPath, wantKey = "/api/v1/services/aigc/multimodal-generation/generation", "plugin-upstream"
					}
					assert.Equal(t, wantPath, r.URL.Path)
					assert.Equal(t, "Bearer "+wantKey, r.Header.Get("Authorization"))
					var received map[string]any
					require.NoError(t, common.DecodeJson(r.Body, &received))
					assert.Equal(t, "qwen-image-3.0", received["model"])
					w.Header().Set("Content-Type", "application/json")
					response := `{"created":1710000000,"data":[{"url":"https://cdn.example/ordinary.png"}]}`
					if tc.plugin {
						response = `{"request_id":"image-local","output":{"choices":[{"message":{"content":[{"image":"https://cdn.example/plugin.png"}]}}]},"usage":{"image_count":1}}`
					} else if publicModel == "qwen-image-3-0" {
						response = `{"id":"local-native-task","status":"queued"}`
					}
					_, err := w.Write([]byte(response))
					require.NoError(t, err)
				}))
				t.Cleanup(upstream.Close)
				mapping := `{"qwen-image-3-0":"qwen-image-3.0"}`
				for _, channel := range []model.Channel{
					{Id: 37, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "ordinary-upstream", Name: "ordinary", Models: publicModel, Group: "default", BaseURL: &upstream.URL, ModelMapping: &mapping},
					{Id: 17, Type: constant.ChannelTypeAli, Status: common.ChannelStatusEnabled, Key: "plugin-upstream", Name: "plugin", Models: publicModel, Group: "cheap", BaseURL: &upstream.URL, ModelMapping: &mapping},
				} {
					require.NoError(t, model.DB.Create(&channel).Error)
					require.NoError(t, channel.UpdateAbilities(nil))
				}
				common.MemoryCacheEnabled = tc.memoryCache
				model.InitChannelCache()
				engine := gin.New()
				SetRelayRouter(engine)
				SetTaskPluginProtocolRouter(engine)
				request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(fmt.Sprintf(`{"model":%q,"prompt":"watercolor","n":1,"size":"1024x1024"}`, publicModel+tc.suffix)))
				if tc.bodyGroup != "" {
					request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(fmt.Sprintf(`{"model":%q,"group":%q,"prompt":"watercolor","n":1,"size":"1024x1024"}`, publicModel+tc.suffix, tc.bodyGroup)))
				}
				request.Header.Set("Authorization", "Bearer "+common.GetStringIfEmpty(tc.authorization, "imageroutingkey"))
				request.Header.Set("X-Omnimux-Group", tc.header)
				request.Header.Set("New-Api-Group", tc.legacyHeader)
				request.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, request)
				require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
				wantCharge := 50000
				if tc.wantGroup == "cheap" {
					wantCharge = 25000
				}
				var user model.User
				require.NoError(t, model.DB.First(&user, 90).Error)
				if tc.wantStatus != 200 {
					assert.Equal(t, 0, calls)
					assert.Equal(t, 10000000, user.Quota)
					return
				}
				assert.Equal(t, 1, calls)
				assert.Equal(t, 10000000-wantCharge, user.Quota)
				if tc.plugin || publicModel == "qwen-image-3-0" {
					var task model.Task
					require.NoError(t, model.DB.First(&task).Error)
					assert.Equal(t, publicModel, task.Properties.OriginModelName)
					wantPlatform := constant.TaskPlatform("55")
					if tc.plugin {
						wantPlatform = constant.TaskPlatform("alibaba")
					}
					assert.Equal(t, wantPlatform, task.Platform)
					assert.Equal(t, tc.wantGroup, task.Group)
					assert.Equal(t, wantCharge, task.Quota)
					if tc.plugin {
						assert.Contains(t, recorder.Body.String(), "https://cdn.example/plugin.png")
					} else {
						assert.Contains(t, recorder.Body.String(), task.TaskID)
					}
				} else {
					assert.Contains(t, recorder.Body.String(), "https://cdn.example/ordinary.png")
				}
			})
		}
	}
}

func TestImageProtocolRouterNativeRetriesKeepCompatibleOwners(t *testing.T) {
	for _, withNativeRetry := range []bool{false, true} {
		t.Run(fmt.Sprint(withNativeRetry), func(t *testing.T) {
			setupImageProtocolRoutingTest(t)
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"qwen-image-3.0":0.1}`))
			previousRetries := common.RetryTimes
			common.RetryTimes = 1
			t.Cleanup(func() { common.RetryTimes = previousRetries })
			calls := map[string]int{}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := r.Header.Get("Authorization")
				calls[key]++
				assert.Equal(t, "/v1/images/generations", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if key == "Bearer first" {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, err := w.Write([]byte(`{"error":{"message":"try another","type":"upstream_error","code":"overloaded"}}`))
					require.NoError(t, err)
					return
				}
				_, err := w.Write([]byte(`{"data":[{"url":"https://cdn.example/retry.png"}]}`))
				require.NoError(t, err)
			}))
			t.Cleanup(upstream.Close)
			for index, key := range []string{"first", "plugin", "native-retry"} {
				if key == "native-retry" && !withNativeRetry {
					continue
				}
				priority := int64(10 - index)
				channelType := constant.ChannelTypeOpenAI
				if key == "plugin" {
					channelType = constant.ChannelTypeAli
				}
				channel := model.Channel{Id: 80 + index, Type: channelType, Status: common.ChannelStatusEnabled, Name: key, Key: key, Models: "qwen-image-3.0", Group: "default", BaseURL: &upstream.URL, Priority: &priority}
				require.NoError(t, model.DB.Create(&channel).Error)
				require.NoError(t, channel.UpdateAbilities(nil))
			}
			model.InitChannelCache()
			engine := gin.New()
			SetRelayRouter(engine)
			SetTaskPluginProtocolRouter(engine)
			request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"qwen-image-3.0","prompt":"test","n":1,"size":"1024x1024"}`))
			request.Header.Set("Authorization", "Bearer imageroutingkey")
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			wantFirstCalls := 2 // The existing selector retries its sole compatible channel.
			if withNativeRetry {
				wantFirstCalls = 1
			}
			assert.Equal(t, wantFirstCalls, calls["Bearer first"])
			assert.Zero(t, calls["Bearer plugin"], "native billing must not become plugin billing during retry")
			var user model.User
			require.NoError(t, model.DB.First(&user, 90).Error)
			if withNativeRetry {
				assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				assert.Equal(t, 1, calls["Bearer native-retry"])
				assert.Equal(t, 9950000, user.Quota)
				assert.Contains(t, recorder.Body.String(), "https://cdn.example/retry.png")
			} else {
				assert.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
				require.Eventually(t, func() bool {
					return model.DB.First(&user, 90).Error == nil && user.Quota == 10000000
				}, time.Second, 5*time.Millisecond, "the native asynchronous refund must restore the complete precharge")
				assert.Equal(t, 10000000, user.Quota)
			}
		})
	}
}

func TestImageProtocolRouterOriginPinCannotBypassTokenModelPermission(t *testing.T) {
	setupImageProtocolRoutingTest(t)
	source, err := builtinplugins.Source("alibaba")
	require.NoError(t, err)
	source = strings.Replace(source, `return { kind: "submit", model: model, action: images.length ? "image_to_image" : "text_to_image", requestBody: requestBody };`, `return { kind: "submit", model: model, originTaskIds: ["image-permission-origin"], action: images.length ? "image_to_image" : "text_to_image", requestBody: requestBody };`, 1)
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	_, err = jsplugin.DefaultRegistry.RegisterFactory(source, jsplugin.Options{Key: "alibaba"})
	require.NoError(t, err)
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)
	channel := model.Channel{Id: 17, Type: constant.ChannelTypeAli, Status: common.ChannelStatusEnabled, Key: "permission-fixture", Name: "permission", Models: "qwen-image-3.0", Group: "default", BaseURL: &upstream.URL}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "image-permission-origin", UserId: 90, ChannelId: 17, Platform: "alibaba", Status: model.TaskStatusSuccess}).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("key = ?", "imageroutingkey").Updates(map[string]any{"model_limits_enabled": true, "model_limits": "different-model"}).Error)
	model.InitChannelCache()
	engine := gin.New()
	SetRelayRouter(engine)
	SetTaskPluginProtocolRouter(engine)
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"qwen-image-3.0","prompt":"test","n":1}`))
	request.Header.Set("Authorization", "Bearer imageroutingkey")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.Zero(t, upstreamCalls)
	var user model.User
	require.NoError(t, model.DB.First(&user, 90).Error)
	assert.Equal(t, 10000000, user.Quota)
	var tasks int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&tasks).Error)
	assert.EqualValues(t, 1, tasks, "permission denial must not create a submitted task")
}

func setupImageProtocolRoutingTest(t *testing.T) {
	t.Helper()
	setupRelayRouterTestDB(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Task{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.Option{}))
	previousPrices := ratio_setting.ModelPrice2JSONString()
	previousGroups := ratio_setting.GroupRatio2JSONString()
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousMemory := common.MemoryCacheEnabled
	previousBatch := common.BatchUpdateEnabled
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"qwen-image":0.1,"qwen-image-edit":0.1,"ordinary-image":0.1,"qwen-image-3-0":0.1,"gpt-image-2.5":0.1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"cheap":0.5}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","cheap":"Cheap"}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		common.MemoryCacheEnabled = previousMemory
		common.BatchUpdateEnabled = previousBatch
	})
	source, err := builtinplugins.Source("alibaba")
	require.NoError(t, err)
	previousRegistry := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	_, err = jsplugin.DefaultRegistry.RegisterFactory(source, jsplugin.Options{Key: "alibaba"})
	require.NoError(t, err)
	t.Cleanup(func() { jsplugin.DefaultRegistry = previousRegistry })
	require.NoError(t, model.DB.Create(&model.User{Id: 90, Username: "image-routing-user", Status: common.UserStatusEnabled, Group: "default", Quota: 10000000}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: 90, Key: "imageroutingkey", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, Group: "default"}).Error)
}
