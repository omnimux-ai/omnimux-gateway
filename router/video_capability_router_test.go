package router

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoCapabilityRoutesSelectAndRetryCompatibleChannels(t *testing.T) {
	h := newVideoCapabilityRouteHarness(t)
	images := make([]string, 10)
	for index := range images {
		pixel := image.NewRGBA(image.Rect(0, 0, 2, 2))
		pixel.Set(0, 0, color.RGBA{R: uint8(index + 1), A: 255})
		var data bytes.Buffer
		require.NoError(t, png.Encode(&data, pixel))
		images[index] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	}

	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("channel-cache=%t", cached), func(t *testing.T) {
			common.MemoryCacheEnabled = cached
			model.InitChannelCache()

			t.Run("catalog publishes each verified reference limit", func(t *testing.T) {
				view := h.musePricing(t)
				for group, expected := range map[string][]int{"default": {2, 10}, "narrow": {2}} {
					var limits []int
					for _, variant := range view.Groups[group] {
						if variant.ID != "video_multi_ref" {
							continue
						}
						assert.Equal(t, "image_reference", variant.ModeKind)
						durationIndex := slices.IndexFunc(variant.Parameters, func(parameter modelcontracts.Parameter) bool {
							return parameter.Name == "duration"
						})
						require.NotEqual(t, -1, durationIndex)
						assert.EqualValues(t, 5, variant.Parameters[durationIndex].Default)
						for _, input := range variant.Inputs {
							if input.Slot == "reference_images" {
								assert.Equal(t, "first_frame", input.FirstItemRole)
								require.NotNil(t, input.Max)
								limits = append(limits, *input.Max)
							}
						}
					}
					assert.ElementsMatch(t, expected, limits, group)
				}
			})

			t.Run("unconfigured model omits the effective view", func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, "/api/model_contract/wan-3.0", nil)
				request.Header.Set("Authorization", "Bearer "+h.browserToken)
				response := httptest.NewRecorder()
				h.engine.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var result struct {
					Data map[string]any `json:"data"`
				}
				require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
				assert.Contains(t, result.Data, "contract")
				assert.NotContains(t, result.Data, "effective_video")
			})

			for _, entry := range []struct{ path, imageField string }{
				{"/v1/videos", "images"},
				{"/v1/video/generations", "image_urls"},
				{"/pg/video/generations", "image_urls"},
			} {
				t.Run(entry.path, func(t *testing.T) {
					ordered := slices.Clone(images)
					if entry.imageField == "image_urls" {
						slices.Reverse(ordered)
					}
					before := h.attemptCount()
					response := h.submit(t, entry.path, "", map[string]any{
						"model": "muse-video", "operation": "video_multi_ref", "prompt": "Animate the ordered images",
						"aspect_ratio": "16:9", entry.imageField: ordered,
					})
					require.Equal(t, http.StatusOK, response.Code, response.Body.String())
					attempts := h.attemptsFrom(before)
					require.Len(t, attempts, 2, "the incompatible channel must receive zero requests")
					for index, key := range []string{"muse-primary", "muse-backup"} {
						assert.Equal(t, key, attempts[index].key)
						assert.Equal(t, "/v1/videos", attempts[index].path)
						body := attempts[index].body
						assert.Equal(t, "muse-video", body["model"])
						assert.EqualValues(t, 5, body["duration"])
						wireImages, ok := body["images"].([]any)
						require.True(t, ok)
						require.Len(t, wireImages, len(ordered))
						for imageIndex, expected := range ordered {
							assert.Equal(t, expected, wireImages[imageIndex], "image at position %d", imageIndex)
						}
						assert.NotContains(t, body, "image")
						assert.NotContains(t, body, "image_urls")
						assert.NotContains(t, body, "operation")
					}
					task := h.latestTask(t)
					assert.Equal(t, h.channels["muse-backup"], task.ChannelId)
					assert.Equal(t, "video_multi_ref", task.Action)
					assert.Equal(t, "default", task.Group)
				})
			}

			t.Run("changed upstream requires a new verified binding", func(t *testing.T) {
				channel := model.Channel{Id: h.channels["muse-primary"]}
				require.NoError(t, model.DB.Model(&channel).UpdateColumn("base_url", h.upstreamURL+"/repointed").Error)
				model.InitChannelCache()
				t.Cleanup(func() {
					assert.NoError(t, model.DB.Model(&channel).UpdateColumn("base_url", h.upstreamURL).Error)
					model.InitChannelCache()
				})
				before := h.attemptCount()
				response := h.submit(t, "/v1/videos", "", map[string]any{
					"model": "muse-video", "prompt": "Animate the ordered images", "images": images,
				})
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				attempts := h.attemptsFrom(before)
				require.Len(t, attempts, 1)
				assert.Equal(t, "muse-backup", attempts[0].key)
			})

			t.Run("single first frame uses the same highest-priority channel", func(t *testing.T) {
				before := h.attemptCount()
				response := h.submit(t, "/v1/videos", "", map[string]any{
					"model": "muse-video", "prompt": "Animate the first frame", "input_reference": images[0],
				})
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				attempts := h.attemptsFrom(before)
				require.Len(t, attempts, 1)
				assert.Equal(t, "muse-narrow", attempts[0].key)
				assert.Equal(t, images[0], attempts[0].body["image"])
				assert.NotContains(t, attempts[0].body, "images")
				assert.Equal(t, "first_frame", h.latestTask(t).Action)
			})

			t.Run("auto skips an incompatible group within the token pool", func(t *testing.T) {
				before := h.attemptCount()
				response := h.submit(t, "/v1/videos", "auto", map[string]any{
					"model": "muse-video", "prompt": "Animate the ordered images", "images": images,
				})
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				attempts := h.attemptsFrom(before)
				require.Len(t, attempts, 2)
				assert.Equal(t, "muse-primary", attempts[0].key)
				assert.Equal(t, "muse-backup", attempts[1].key)
				assert.Equal(t, "default", h.latestTask(t).Group)
			})

			t.Run("auto exhaustion preserves the last upstream rate limit", func(t *testing.T) {
				before := h.musePricing(t)
				t.Cleanup(func() {
					for _, key := range []string{"muse-primary", "muse-backup"} {
						channel, err := model.GetChannelById(h.channels[key], true)
						require.NoError(t, err)
						if channel.Status != common.ChannelStatusEnabled {
							assert.True(t, model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusEnabled, "fixture restored"))
						}
					}
					model.InitChannelCache()
				})
				require.True(t, model.UpdateChannelStatus(h.channels["muse-backup"], "", common.ChannelStatusManuallyDisabled, "fixture backup disabled"))
				start := h.attemptCount()
				response := h.submit(t, "/v1/videos", "auto", map[string]any{
					"model": "muse-video", "prompt": pauseMusePrimaryPrompt, "images": images,
				})
				require.Equal(t, http.StatusTooManyRequests, response.Code, response.Body.String())
				attempts := h.attemptsFrom(start)
				require.Len(t, attempts, 1)
				assert.Equal(t, "muse-primary", attempts[0].key)
				after := h.musePricing(t)
				assert.NotEqual(t, before.Version, after.Version)
				assert.Equal(t, after.Groups["narrow"], after.Groups["default"])
			})

			for _, selection := range []struct{ name, model, token string }{
				{"explicit group", "muse-video@narrow", ""},
				{"token auto pool", "muse-video", "restricted"},
				{"admin channel pin", "muse-video", "pin"},
			} {
				t.Run(selection.name+" rejects before submission", func(t *testing.T) {
					h.requireMismatch(t, "/v1/videos", selection.token, map[string]any{
						"model": selection.model, "prompt": "Animate all references", "images": images,
					})
				})
			}

			t.Run("Seedance variants preserve frame and reference roles", func(t *testing.T) {
				for _, path := range []string{"/v1/video/generations", "/pg/video/generations"} {
					firstLastFrames := []map[string]any{
						{"role": "first_frame", "url": "https://media.example/first.png"},
						{"role": "last_frame", "url": "https://media.example/last.png"},
					}
					referenceImages := []string{"https://media.example/reference.png"}
					referenceVideos := []string{"https://media.example/source.mp4"}
					for _, tc := range []struct {
						mode, taskType, resolution string
						duration                   int
						media                      map[string]any
						want                       map[string]any
					}{
						{
							mode: "first_last_frame", resolution: "720p", duration: 5,
							media: map[string]any{"image_with_roles": firstLastFrames},
							want:  map[string]any{"image_with_roles": []any{firstLastFrames[0], firstLastFrames[1]}},
						},
						{
							mode: "video_multi_ref", taskType: "reference", resolution: "480p", duration: 5,
							media: map[string]any{"images": referenceImages, "video_urls": referenceVideos},
							want:  map[string]any{"image_urls": []any{referenceImages[0]}, "video_urls": []any{referenceVideos[0]}},
						},
						{
							mode: "video_edit", taskType: "edit", resolution: "720p", duration: -1,
							media: map[string]any{"video_urls": referenceVideos},
							want:  map[string]any{"video_urls": []any{referenceVideos[0]}},
						},
						{
							mode: "video_extend", taskType: "extend", resolution: "720p", duration: 5,
							media: map[string]any{"video_urls": referenceVideos},
							want:  map[string]any{"video_urls": []any{referenceVideos[0]}},
						},
					} {
						t.Run(path+"/"+tc.mode, func(t *testing.T) {
							before := h.attemptCount()
							request := tc.media
							request["model"] = "seedance-2-5"
							request["prompt"] = "Make a video"
							request["operation"] = tc.mode
							request["resolution"] = tc.resolution
							request["seconds"] = tc.duration
							response := h.submit(t, path, "", request)
							require.Equal(t, http.StatusOK, response.Code, response.Body.String())
							attempts := h.attemptsFrom(before)
							require.Len(t, attempts, 1)
							assert.Equal(t, "seedance", attempts[0].key)
							assert.Equal(t, "/v1/videos/generations", attempts[0].path)
							body := attempts[0].body
							assert.Equal(t, "seedance-2.5", body["model"])
							assert.Equal(t, tc.resolution, body["resolution"])
							assert.EqualValues(t, tc.duration, body["duration"])
							assert.NotContains(t, body, "operation")
							assert.NotContains(t, body, "seconds")
							if tc.taskType != "" {
								assert.Equal(t, tc.taskType, body["omni_reference_task_type"])
							}
							for field, value := range tc.want {
								assert.Equal(t, value, body[field])
							}
						})
					}

					t.Run(path+"/reject a combination across variants", func(t *testing.T) {
						h.requireMismatch(t, path, "", map[string]any{
							"model": "seedance-2-5", "operation": "video_multi_ref", "prompt": "Make a video", "duration": 5,
							"resolution": "720p", "omni_reference_task_type": "reference", "image_urls": referenceImages, "video_urls": referenceVideos,
						})
					})
				}
			})

			t.Run("plugin override cannot inherit verified capabilities", func(t *testing.T) {
				before := h.musePricing(t)
				source, err := builtinplugins.Source("sora")
				require.NoError(t, err)
				_, err = jsplugin.DefaultRegistry.Register(source, jsplugin.Options{Key: "sora"})
				require.NoError(t, err)
				t.Cleanup(func() { assert.NoError(t, jsplugin.DefaultRegistry.Unregister("sora")) })
				start := h.attemptCount()
				response := h.submit(t, "/v1/videos", "", map[string]any{
					"model": "muse-video", "prompt": "Animate the ordered images", "images": images,
				})
				require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
				assert.Equal(t, start, h.attemptCount())
				after := h.musePricing(t)
				assert.NotEqual(t, before.Version, after.Version)
				assert.Equal(t, []modelcontracts.Operation{}, after.Groups["default"])
				assert.Equal(t, []modelcontracts.Operation{}, after.Groups["narrow"])
			})
		})
	}
}

const pauseMusePrimaryPrompt = "Pause the only compatible upstream"

type videoCapabilityAttempt struct {
	key  string
	path string
	body map[string]any
}

type videoCapabilityRouteHarness struct {
	engine       *gin.Engine
	userID       int
	browserToken string
	upstreamURL  string
	tokens       map[string]string
	channels     map[string]int
	mu           sync.Mutex
	attempts     []videoCapabilityAttempt
}

func newVideoCapabilityRouteHarness(t *testing.T) *videoCapabilityRouteHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())

	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldDBType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	oldMaster, oldRedis := common.IsMasterNode, common.RedisEnabled
	oldMemory, oldBatch := common.MemoryCacheEnabled, common.BatchUpdateEnabled
	oldSQLitePath, oldSessionSecret := common.SQLitePath, common.SessionSecret
	oldRetry, oldRegistry := common.RetryTimes, jsplugin.DefaultRegistry
	oldPrices := ratio_setting.ModelPrice2JSONString()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldGroups := setting.UserUsableGroups2JSONString()
	oldCapabilities := modelcontracts.ActiveVideoCapabilities()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldDBType, oldLogType)
		common.IsMasterNode, common.RedisEnabled = oldMaster, oldRedis
		common.MemoryCacheEnabled, common.BatchUpdateEnabled = oldMemory, oldBatch
		common.SQLitePath, common.SessionSecret = oldSQLitePath, oldSessionSecret
		common.RetryTimes, jsplugin.DefaultRegistry = oldRetry, oldRegistry
		modelcontracts.SetVideoCapabilities(oldCapabilities)
		model.InvalidatePricingCache()
		assert.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrices))
		assert.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		assert.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldGroups))
	})

	common.IsMasterNode, common.RedisEnabled = false, false
	common.MemoryCacheEnabled, common.BatchUpdateEnabled = false, false
	common.SQLitePath = t.TempDir() + "/video-capabilities.db"
	common.SessionSecret = "video-capability-route-test-secret"
	common.RetryTimes = 1
	// This variable is reserved for disposable integration test databases.
	t.Setenv("SQL_DSN", os.Getenv("OMNIMUX_VIDEO_TEST_SQL_DSN"))
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitDB())
	db := model.DB
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	require.NoError(t, model.InitLogDB())
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.UserSession{}, &model.Channel{},
		&model.Ability{}, &model.Task{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.Option{}, &model.Model{}, &model.Vendor{}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"muse-video":0.01,"seedance-2-5":0.01}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"narrow":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","narrow":"Narrow","auto":"Automatic"}`))

	service.InitHttpClient()
	source, err := builtinplugins.Source("sora")
	require.NoError(t, err)
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	_, err = jsplugin.DefaultRegistry.RegisterFactory(source, jsplugin.Options{Key: "sora"})
	require.NoError(t, err)

	suffix := common.GetRandomString(10)
	user := model.User{
		Username: "video-" + suffix, Role: common.RoleAdminUser, Group: "default",
		Status: common.UserStatusEnabled, Quota: 10000000, AuthVersion: 1, AffCode: suffix,
	}
	require.NoError(t, db.Create(&user).Error)
	session := model.UserSession{
		SID: "video-session-" + suffix, UserID: user.Id, Version: 1, UserAuthVersion: 1, LoginMethod: "password",
		Status: model.UserSessionStatusActive, RefreshHash: "video-refresh-" + suffix,
		LastActiveAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, model.CreateUserSession(&session))
	browserToken, _, err := service.IssueAccessToken(service.AuthIdentity{
		UserID: user.Id, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1,
	})
	require.NoError(t, err)
	h := &videoCapabilityRouteHarness{
		engine: gin.New(), userID: user.Id, browserToken: browserToken, tokens: map[string]string{}, channels: map[string]int{},
	}
	for _, tc := range []struct{ name, group, autoGroups string }{
		{"", "default", ""}, {"auto", "auto", `["narrow","default"]`}, {"restricted", "auto", `["narrow"]`},
	} {
		token := model.Token{
			Name: "video test", Key: "video" + tc.name + suffix, UserId: user.Id, Status: common.TokenStatusEnabled,
			ExpiredTime: -1, UnlimitedQuota: true, Group: tc.group, AutoGroups: tc.autoGroups,
		}
		require.NoError(t, db.Create(&token).Error)
		h.tokens[tc.name] = "sk-" + token.Key
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := common.DecodeJson(r.Body, &body); err != nil {
			t.Errorf("mock upstream got an invalid JSON request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		h.mu.Lock()
		h.attempts = append(h.attempts, videoCapabilityAttempt{key: key, path: r.URL.Path, body: body})
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if key == "muse-narrow" {
			if images, _ := body["images"].([]any); len(images) > 2 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"message":"At most two references accepted"}}`)
				return
			}
		}
		if key == "muse-primary" {
			if body["prompt"] == pauseMusePrimaryPrompt {
				assert.True(t, model.UpdateChannelStatus(h.channels[key], "", common.ChannelStatusManuallyDisabled, "fixture primary disabled"))
			}
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"Retry on a compatible channel"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"synthetic-video-task","status":"queued"}`)
	}))
	t.Cleanup(upstream.Close)
	h.upstreamURL = upstream.URL

	for _, tc := range []struct {
		key, publicModel, group string
		channelType             int
		priority                int64
	}{
		{"muse-narrow", "muse-video", "default,narrow", constant.ChannelTypeSora, 30},
		{"muse-primary", "muse-video", "default", constant.ChannelTypeSora, 20},
		{"muse-backup", "muse-video", "default", constant.ChannelTypeSora, 10},
		{"seedance", "seedance-2-5", "default", constant.ChannelTypeApiMart, 10},
	} {
		channel := model.Channel{
			Type: tc.channelType, Key: tc.key, Name: tc.key, Models: tc.publicModel, Group: tc.group,
			BaseURL: &upstream.URL, Status: common.ChannelStatusEnabled, Priority: &tc.priority, AutoBan: common.GetPointer(0),
		}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
		h.channels[tc.key] = channel.Id
	}
	h.tokens["pin"] = h.tokens[""] + "-" + fmt.Sprint(h.channels["muse-narrow"])
	rules, err := modelcontracts.ReadVideoCapabilities(strings.NewReader(fmt.Sprintf(`{"models":{
		"muse-video":[
			{"channel_id":%[2]d,"channel_type":%[6]d,"profile":"openai_video","upstream_model":"muse-video","base_url":%[1]q,"evidence":"synthetic upstream",
			 "operations":[{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","min":2,"max":2}]},{"id":"first_frame"}]},
			{"channel_id":%[3]d,"channel_type":%[6]d,"profile":"openai_video","upstream_model":"muse-video","base_url":%[1]q,"evidence":"synthetic upstream",
			 "operations":[{"id":"video_multi_ref"}]},
			{"channel_id":%[4]d,"channel_type":%[6]d,"profile":"openai_video","upstream_model":"muse-video","base_url":%[1]q,"evidence":"synthetic upstream",
			 "operations":[{"id":"video_multi_ref"}]}
		],
		"seedance-2-5":[
			{"channel_id":%[5]d,"channel_type":%[7]d,"profile":"apimart_video","upstream_model":"seedance-2-5","base_url":%[1]q,"evidence":"synthetic upstream",
			 "operations":[{"id":"first_last_frame"},{"id":"video_edit"},{"id":"video_extend"},
				{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true}],
				 "parameters":[{"name":"duration"},{"name":"resolution","enum":["720p"]},{"name":"aspect_ratio"},{"name":"generate_audio"}]},
				{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true},{"slot":"reference_videos","min":1,"required":true}],
				 "parameters":[{"name":"duration"},{"name":"resolution","enum":["480p"]},{"name":"aspect_ratio"},{"name":"generate_audio"}]}]}
		]
	}}`, upstream.URL, h.channels["muse-narrow"], h.channels["muse-primary"], h.channels["muse-backup"], h.channels["seedance"], constant.ChannelTypeSora, constant.ChannelTypeApiMart)))
	require.NoError(t, err)
	modelcontracts.SetVideoCapabilities(rules)

	SetRelayRouter(h.engine)
	SetApiRouter(h.engine)
	SetVideoRouter(h.engine)
	SetTaskPluginProtocolRouter(h.engine)
	return h
}

func (h *videoCapabilityRouteHarness) submit(t *testing.T, path, token string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	credential := h.tokens[token]
	if strings.HasPrefix(path, "/pg/") {
		credential = h.browserToken
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	h.engine.ServeHTTP(recorder, request)
	return recorder
}

func (h *videoCapabilityRouteHarness) musePricing(t *testing.T) *model.EffectiveVideo {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	request.Header.Set("Authorization", "Bearer "+h.browserToken)
	response := httptest.NewRecorder()
	h.engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var catalog struct {
		Data []model.Pricing `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &catalog))
	index := slices.IndexFunc(catalog.Data, func(row model.Pricing) bool { return row.ModelName == "muse-video" })
	require.NotEqual(t, -1, index)
	view := catalog.Data[index].EffectiveVideo
	require.NotNil(t, view)
	return view
}

func (h *videoCapabilityRouteHarness) attemptCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.attempts)
}

func (h *videoCapabilityRouteHarness) attemptsFrom(start int) []videoCapabilityAttempt {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.attempts[start:])
}

func (h *videoCapabilityRouteHarness) latestTask(t *testing.T) model.Task {
	t.Helper()
	var task model.Task
	require.NoError(t, model.DB.Where("user_id = ?", h.userID).Last(&task).Error)
	return task
}

func (h *videoCapabilityRouteHarness) requireMismatch(t *testing.T, path, token string, request map[string]any) {
	t.Helper()
	attempts := h.attemptCount()
	before, err := model.GetUserQuota(h.userID, true)
	require.NoError(t, err)
	var tasksBefore int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("user_id = ?", h.userID).Count(&tasksBefore).Error)
	response := h.submit(t, path, token, request)
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	var result struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	assert.Equal(t, "video_capability_mismatch", result.Error.Code)
	assert.Equal(t, attempts, h.attemptCount(), "incompatible work must not reach an upstream")
	after, err := model.GetUserQuota(h.userID, true)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	var tasksAfter int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("user_id = ?", h.userID).Count(&tasksAfter).Error)
	assert.Equal(t, tasksBefore, tasksAfter)
}
