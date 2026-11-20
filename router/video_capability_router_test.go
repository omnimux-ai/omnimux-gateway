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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoCapabilityRoutesSelectAndRetryCompatibleChannels(t *testing.T) {
	h := newVideoCapabilityRouteHarness(t)
	images := videoCapabilityReferenceImages(t, 10)

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
				const path, token = "/v1/videos", "auto"
				balances, err := h.readBalances(path, token)
				require.NoError(t, err)
				// A failed submission refunds asynchronously. Wait for both refunds before the next subtest reads this wallet.
				t.Cleanup(func() {
					require.EventuallyWithT(t, func(c *assert.CollectT) {
						after, err := h.readBalances(path, token)
						if assert.NoError(c, err) {
							assert.Equal(c, balances, after)
						}
					}, time.Second, 5*time.Millisecond, "failed submission must restore the wallet and token balances")
				})
				start := h.attemptCount()
				response := h.submit(t, path, token, map[string]any{
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

const (
	syntheticGroupedVideoModel = "example-group-video"
	syntheticMixedVideoModel   = "example-mixed-video"
	syntheticVideoBytes        = "synthetic video artifact"
)

func TestRegisteredVideoProtocolCatalogAndRoutes(t *testing.T) {
	// Loading a spec directory is a startup-only operation that cannot be
	// restored through the public API. Keep the synthetic spec process-local.
	const childEnv = "OMNIMUX_TEST_GROUPED_VIDEO_CHILD"
	if os.Getenv(childEnv) != "1" {
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRegisteredVideoProtocolCatalogAndRoutes$")
		command.Env = append(os.Environ(), childEnv+"=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}
	h := newVideoCapabilityRouteHarness(t)
	t.Run("native", func(t *testing.T) { assertNativeGroupedVideoRoutes(t, h) })
	t.Run("mixed", func(t *testing.T) { assertMixedGroupedVideoRoutes(t, h) })
}

func assertNativeGroupedVideoRoutes(t *testing.T, h *videoCapabilityRouteHarness) {
	t.Helper()
	specDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(specDir, syntheticGroupedVideoModel+".json"), []byte(`{
		"model_id":"example-group-video","family":"video","vendor":"Example","status":"partial",
		"gateway":{"profile":"apimart_video","request_model":"seedance-2-5","operations":["first_frame","video_multi_ref"],
			"mode_kinds":{"first_frame":"first_frame","video_multi_ref":"image_reference"}},
		"reference_images":{"min":1,"max":4,"formats":["image/png"]},
		"durations_seconds":{"min":4,"max":6},"resolutions":["480p","720p"],
		"aspect_ratios":["adaptive","16:9"],"output_formats":["mp4"]
	}`), 0o600))
	require.NoError(t, modelcontracts.ApplySpecDir(specDir))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"example-group-video":0.001}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"narrow":1,"fixture-hidden":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","narrow":"Narrow","auto":"Automatic"}`))

	const firstFrame = `{"id":"first_frame"}`
	const multiRef = `{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":%d}],
		"parameters":[{"name":"duration"},{"name":"resolution","enum":[%q]}]}`
	twoReferences := fmt.Sprintf(multiRef, 2, "720p")
	fourReferences := fmt.Sprintf(multiRef, 4, "480p")
	var channelRules []common.RawMessage
	for _, item := range []struct {
		key, group, variants string
		priority             int64
	}{
		{"example-default", "default", firstFrame + "," + twoReferences + "," + fourReferences, 100},
		{"example-narrow-primary", "narrow", firstFrame + "," + twoReferences, 20},
		{"example-narrow-backup", "narrow", firstFrame + "," + twoReferences, 10},
		{"example-hidden", "fixture-hidden", firstFrame, 50},
	} {
		id := h.addSyntheticChannel(t, item.key, syntheticGroupedVideoModel, item.group, "example-upstream-video", constant.ChannelTypeApiMart, item.priority)
		channelRules = append(channelRules, common.RawMessage(fmt.Sprintf(`{
			"channel_id":%d,"channel_type":%d,"profile":"apimart_video","request_model":"seedance-2-5",
			"upstream_model":"example-upstream-video","base_url":%q,"evidence":"synthetic upstream",
			"operations":[%s]}`, id, constant.ChannelTypeApiMart, h.upstreamURL, item.variants)))
	}
	rawRules, err := common.Marshal(map[string]any{"models": map[string][]common.RawMessage{syntheticGroupedVideoModel: channelRules}})
	require.NoError(t, err)
	rules, err := modelcontracts.ReadVideoCapabilities(bytes.NewReader(rawRules))
	require.NoError(t, err)
	modelcontracts.SetVideoCapabilities(rules)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	assertCatalog := func() {
		t.Helper()
		pricing, contract := h.syntheticCatalogResponses(t, syntheticGroupedVideoModel)
		data := contract["data"].(map[string]any)
		row := pricing["data"].([]map[string]any)[0]
		assert.Equal(t, "video_generations", data["contract"].(map[string]any)["gateway_protocol"])
		assert.Equal(t, data["effective_video"], row["effective_video"])
		groups := data["effective_video"].(map[string]any)["groups"].(map[string]any)
		require.Len(t, groups, 2)
		assert.NotContains(t, groups, "fixture-hidden")
		for groupName, expected := range map[string]map[string]float64{"default": {"480p": 4, "720p": 2}, "narrow": {"720p": 2}} {
			require.Contains(t, groups, groupName)
			variants := groups[groupName].([]any)
			limits := map[string]float64{}
			firstFrames := 0
			for _, value := range variants {
				operation := value.(map[string]any)
				if operation["id"] == "first_frame" {
					firstFrames++
					continue
				}
				assert.Equal(t, "video_multi_ref", operation["id"])
				input := operation["inputs"].([]any)[1].(map[string]any)
				assert.Equal(t, "reference_images", input["slot"])
				resolution := operation["parameters"].([]any)[1].(map[string]any)
				assert.Equal(t, "resolution", resolution["name"])
				limits[resolution["enum"].([]any)[0].(string)] = input["max"].(float64)
			}
			assert.Equal(t, 1, firstFrames)
			assert.Equal(t, expected, limits)
		}
		writeGroupedVideoCatalogFixture(t, "catalog.json", map[string]any{
			"/api/pricing": pricing, "/api/model_contract/" + syntheticGroupedVideoModel: contract,
		})
	}
	assertCatalog()
	common.MemoryCacheEnabled = true
	model.InitChannelCache()
	assertCatalog()

	const path = "/v1/video/generations"
	references := []string{"https://media.example/first.png", "https://media.example/second.png", "https://media.example/third.png"}
	frames := []map[string]any{{"role": "first_frame", "url": references[0]}}
	beforeFrame := h.attemptCount()
	first := h.submit(t, path, "", map[string]any{
		"model": syntheticGroupedVideoModel + "@default", "prompt": "Animate the first frame",
		"operation": "first_frame", "duration": 5, "resolution": "720p", "image_with_roles": frames,
	})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	frameAttempts := h.attemptsFrom(beforeFrame)
	require.Len(t, frameAttempts, 1)
	assert.Equal(t, "example-default", frameAttempts[0].key)
	assert.Equal(t, []any{frames[0]}, frameAttempts[0].body["image_with_roles"])
	submission := func(group, resolution string, images []string) map[string]any {
		return map[string]any{"model": syntheticGroupedVideoModel + "@" + group, "prompt": "Animate the examples",
			"operation": "video_multi_ref", "duration": 5, "resolution": resolution, "images": images}
	}
	h.requireMismatch(t, path, "", submission("default", "720p", references))
	missingDuration := submission("default", "720p", references[:2])
	delete(missingDuration, "duration")
	h.requireMismatch(t, path, "", missingDuration)
	wideStart := h.attemptCount()
	wide := h.submit(t, path, "", submission("default", "480p", references))
	require.Equal(t, http.StatusOK, wide.Code, wide.Body.String())
	wideAttempts := h.attemptsFrom(wideStart)
	require.Len(t, wideAttempts, 1)
	assert.Equal(t, "example-default", wideAttempts[0].key)
	assert.Equal(t, []any{references[0], references[1], references[2]}, wideAttempts[0].body["image_urls"])
	before := h.attemptCount()
	forbidden := h.submit(t, path, "", submission("fixture-hidden", "720p", references[:2]))
	require.Equal(t, http.StatusForbidden, forbidden.Code, forbidden.Body.String())
	assert.Equal(t, before, h.attemptCount())

	// The token's default group also has a compatible channel. An explicit
	// suffix must still pin both attempts to the narrow group.
	response := h.submit(t, path, "", submission("narrow", "720p", references[:2]))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	attempts := h.attemptsFrom(before)
	require.Len(t, attempts, 2, "both attempts must stay in the selected group")
	for i, key := range []string{"example-narrow-primary", "example-narrow-backup"} {
		assert.Equal(t, key, attempts[i].key)
		assert.Equal(t, "/v1/videos/generations", attempts[i].path)
		assert.Equal(t, "example-upstream-video", attempts[i].body["model"])
		assert.Equal(t, []any{references[0], references[1]}, attempts[i].body["image_urls"])
		assert.Equal(t, "720p", attempts[i].body["resolution"])
	}
	task := h.latestTask(t)
	require.NotEmpty(t, task.TaskID)
	assert.Equal(t, "narrow", task.Group)
	assert.Equal(t, h.channels["example-narrow-backup"], task.ChannelId)

	// Revoke future route variants while preserving the accepted task. The
	// worker must poll its saved upstream ID/channel and the public endpoints
	// must retrieve the original result.
	modelcontracts.SetVideoCapabilities(nil)
	model.InvalidatePricingCache()
	beforeRevoked := h.attemptCount()
	revoked := h.submit(t, path, "", submission("narrow", "720p", references[:2]))
	require.Equal(t, http.StatusServiceUnavailable, revoked.Code, revoked.Body.String())
	assert.Equal(t, beforeRevoked, h.attemptCount())
	pollStart := h.attemptCount()
	originalResolver := service.GetTaskAdaptorFunc
	t.Cleanup(func() { service.GetTaskAdaptorFunc = originalResolver })
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		adaptor := relay.GetTaskAdaptor(platform)
		if adaptor == nil {
			return nil
		}
		return adaptor
	}
	upstreamID := task.GetUpstreamTaskID()
	require.NoError(t, service.UpdateVideoTasks(t.Context(), task.Platform, map[int][]string{task.ChannelId: {upstreamID}}, map[string]*model.Task{upstreamID: &task}))
	polls := h.attemptsFrom(pollStart)
	require.Len(t, polls, 1)
	assert.Equal(t, "example-narrow-backup", polls[0].key)
	assert.Equal(t, "/v1/tasks/"+upstreamID, polls[0].path)

	query := h.get(t, path+"/"+task.TaskID, h.tokens[""].bearer)
	require.Equal(t, http.StatusOK, query.Code, query.Body.String())
	var result struct {
		Data struct {
			TaskID string `json:"task_id"`
			Group  string `json:"group"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(query.Body.Bytes(), &result))
	assert.Equal(t, task.TaskID, result.Data.TaskID)
	assert.Equal(t, "narrow", result.Data.Group)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Data.Status)

	// The fixture clip is served by httptest on localhost with no credentials.
	upstream, err := url.Parse(h.upstreamURL)
	require.NoError(t, err)
	require.NotEmpty(t, upstream.Port())
	fetch := system_setting.GetFetchSetting()
	previousFetchSetting := *fetch
	fetch.AllowPrivateIp = true
	fetch.AllowedPorts = []string{upstream.Port()}
	t.Cleanup(func() { *fetch = previousFetchSetting })
	clip := h.get(t, "/v1/videos/"+task.TaskID+"/content", h.tokens[""].bearer)
	require.Equal(t, http.StatusOK, clip.Code, clip.Body.String())
	assert.Equal(t, "video/mp4", clip.Header().Get("Content-Type"))
	assert.Equal(t, syntheticVideoBytes, clip.Body.String())
}

func assertMixedGroupedVideoRoutes(t *testing.T, h *videoCapabilityRouteHarness) {
	t.Helper()
	specDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(specDir, syntheticMixedVideoModel+".json"), []byte(`{
		"model_id":"example-mixed-video","family":"video","vendor":"Example","status":"partial",
		"gateway":{"profile":"openai_video","operations":["first_frame","video_multi_ref"],
			"mode_kinds":{"first_frame":"first_frame","video_multi_ref":"image_reference"}},
		"reference_images":{"min":2,"max":4,"formats":["image/png"]},
		"durations_seconds":{"min":4,"max":6},"aspect_ratios":["16:9","adaptive"],"output_formats":["mp4"]
	}`), 0o600))
	require.NoError(t, modelcontracts.ApplySpecDir(specDir))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"example-mixed-video":0.001}`))
	nativeID := h.addSyntheticChannel(t, "example-mixed-native", syntheticMixedVideoModel, "default", "example-upstream-video", constant.ChannelTypeApiMart, 10)
	openAIID := h.addSyntheticChannel(t, "example-mixed-ordered", syntheticMixedVideoModel, "narrow", "muse-video", constant.ChannelTypeSora, 10)
	rawRules := fmt.Sprintf(`{"models":{"example-mixed-video":[
		{"channel_id":%d,"channel_type":%d,"profile":"apimart_video","request_model":"seedance-2-5","upstream_model":"example-upstream-video",
		 "base_url":%q,"evidence":"synthetic upstream","operations":[{"id":"first_frame"}]},
		{"channel_id":%d,"channel_type":%d,"profile":"openai_video","upstream_model":"muse-video","base_url":%q,"evidence":"synthetic upstream",
		 "operations":[{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","max":2}]}]}
	]}}`, nativeID, constant.ChannelTypeApiMart, h.upstreamURL, openAIID, constant.ChannelTypeSora, h.upstreamURL)
	rules, err := modelcontracts.ReadVideoCapabilities(strings.NewReader(rawRules))
	require.NoError(t, err)
	modelcontracts.SetVideoCapabilities(rules)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	pricing, contract := h.syntheticCatalogResponses(t, syntheticMixedVideoModel)
	row := pricing["data"].([]map[string]any)[0]
	data := contract["data"].(map[string]any)
	assert.Equal(t, "openai_video", data["contract"].(map[string]any)["gateway_protocol"])
	assert.Equal(t, data["effective_video"], row["effective_video"])
	groups := data["effective_video"].(map[string]any)["groups"].(map[string]any)
	require.Len(t, groups, 2)
	for _, item := range []struct{ group, protocol, operation string }{
		{"default", "video_generations", "first_frame"},
		{"narrow", "openai_video", "video_multi_ref"},
	} {
		variants := groups[item.group].([]any)
		require.Len(t, variants, 1)
		variant := variants[0].(map[string]any)
		assert.Equal(t, item.protocol, variant["gateway_protocol"])
		assert.Equal(t, item.operation, variant["id"])
	}
	writeGroupedVideoCatalogFixture(t, "mixed-catalog.json", map[string]any{
		"/api/pricing": pricing, "/api/model_contract/" + syntheticMixedVideoModel: contract,
	})

	const nativePath = "/v1/video/generations"
	before := h.attemptCount()
	nativeResponse := h.submitWithProtocol(t, nativePath, "", "video_generations", map[string]any{
		"model": syntheticMixedVideoModel + "@default", "prompt": "Animate the first frame", "operation": "first_frame",
		"duration": 5, "image_with_roles": []map[string]string{{"role": "first_frame", "url": "https://media.example/first.png"}},
	})
	require.Equal(t, http.StatusOK, nativeResponse.Code, nativeResponse.Body.String())
	attempts := h.attemptsFrom(before)
	require.Len(t, attempts, 1)
	assert.Equal(t, "example-mixed-native", attempts[0].key)
	assert.Equal(t, "/v1/videos/generations", attempts[0].path)

	images := videoCapabilityReferenceImages(t, 2)
	wrongEndpoint := h.submitWithProtocol(t, "/v1/videos", "", "video_generations", map[string]any{
		"model": syntheticMixedVideoModel + "@default", "prompt": "Use an unsupported protocol on this endpoint",
	})
	require.Equal(t, http.StatusBadRequest, wrongEndpoint.Code, wrongEndpoint.Body.String())
	assert.Equal(t, before+1, h.attemptCount())
	for _, path := range []string{"/v1/videos", nativePath} {
		before = h.attemptCount()
		// The older shared-path caller omits the protocol header and inherits the
		// model-level OpenAI wire format, even with a native group present.
		response := h.submit(t, path, "", map[string]any{
			"model": syntheticMixedVideoModel + "@narrow", "prompt": "Animate both references", "images": images,
		})
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		attempts = h.attemptsFrom(before)
		require.Len(t, attempts, 1)
		assert.Equal(t, "example-mixed-ordered", attempts[0].key)
		assert.Equal(t, "/v1/videos", attempts[0].path)
		assert.Equal(t, []any{images[0], images[1]}, attempts[0].body["images"])
		task := h.latestTask(t)
		assert.Equal(t, "narrow", task.Group)
		query := h.get(t, "/v1/videos/"+task.TaskID, h.tokens[""].bearer)
		require.Equal(t, http.StatusOK, query.Code, query.Body.String())
		var result struct {
			ID string `json:"id"`
		}
		require.NoError(t, common.Unmarshal(query.Body.Bytes(), &result))
		assert.Equal(t, task.TaskID, result.ID)
	}
}

func videoCapabilityReferenceImages(t *testing.T, count int) []string {
	t.Helper()
	images := make([]string, count)
	for index := range images {
		pixel := image.NewRGBA(image.Rect(0, 0, 2, 2))
		pixel.Set(0, 0, color.RGBA{R: uint8(index + 1), A: 255})
		var data bytes.Buffer
		require.NoError(t, png.Encode(&data, pixel))
		images[index] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	}
	return images
}

func (h *videoCapabilityRouteHarness) addSyntheticChannel(t *testing.T, key, publicModel, group, upstreamModel string, channelType int, priority int64) int {
	t.Helper()
	mapping, err := common.Marshal(map[string]string{publicModel: upstreamModel})
	require.NoError(t, err)
	channel := model.Channel{
		Type: channelType, Key: key, Name: key, Models: publicModel, Group: group,
		BaseURL: &h.upstreamURL, Status: common.ChannelStatusEnabled, Priority: &priority, AutoBan: common.GetPointer(0),
		ModelMapping: common.GetPointer(string(mapping)),
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	h.channels[key] = channel.Id
	return channel.Id
}

func (h *videoCapabilityRouteHarness) syntheticCatalogResponses(t *testing.T, modelID string) (map[string]any, map[string]any) {
	t.Helper()
	var pricing, contract map[string]any
	for path, target := range map[string]*map[string]any{"/api/pricing": &pricing, "/api/model_contract/" + modelID: &contract} {
		reply := h.get(t, path, h.browserToken)
		require.Equal(t, http.StatusOK, reply.Code, reply.Body.String())
		assert.Equal(t, "private, no-store", reply.Header().Get("Cache-Control"))
		for _, private := range []string{h.upstreamURL, "example-upstream-video", "example-narrow-primary", "example-mixed-native", "synthetic upstream"} {
			assert.NotContains(t, reply.Body.String(), private)
		}
		require.NoError(t, common.Unmarshal(reply.Body.Bytes(), target))
	}
	var rows []map[string]any
	for _, value := range pricing["data"].([]any) {
		row := value.(map[string]any)
		if row["model_name"] == modelID {
			// The legacy pricing marker belongs to the first row before
			// filtering and can land on a model outside this fixture.
			delete(row, "pricing_version")
			// Group ordering is independent of these capability fixtures.
			groups := row["enable_groups"].([]any)
			slices.SortFunc(groups, func(a, b any) int { return strings.Compare(a.(string), b.(string)) })
			rows = append(rows, row)
		}
	}
	require.Len(t, rows, 1)
	pricing["data"] = rows
	return pricing, contract
}

func writeGroupedVideoCatalogFixture(t *testing.T, name string, responses map[string]any) {
	t.Helper()
	raw, err := common.Marshal(responses)
	require.NoError(t, err)
	formatted, err := common.IndentJson(raw)
	require.NoError(t, err)
	path := filepath.Join("testdata", "issue-74", name)
	// Keep regular test runs read-only. Regeneration uses the same handlers and
	// assertions as verification.
	if os.Getenv("OMNIMUX_UPDATE_GROUPED_VIDEO_FIXTURES") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, append(formatted, '\n'), 0o644))
		return
	}
	expected, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), string(formatted), "synthetic fixture must agree with the HTTP catalog")
}

type videoCapabilityAttempt struct {
	key  string
	path string
	body map[string]any
}

type videoCapabilityRouteToken struct {
	id     int
	bearer string
}

type videoCapabilityRouteBalances struct {
	wallet         int
	tokenRemaining int
	tokenUsed      int
}

type videoCapabilityRouteHarness struct {
	engine       *gin.Engine
	userID       int
	browserToken string
	upstreamURL  string
	tokens       map[string]videoCapabilityRouteToken
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
		engine: gin.New(), userID: user.Id, browserToken: browserToken, tokens: map[string]videoCapabilityRouteToken{}, channels: map[string]int{},
	}
	for _, tc := range []struct{ name, group, autoGroups string }{
		{"", "default", ""}, {"auto", "auto", `["narrow","default"]`}, {"restricted", "auto", `["narrow"]`},
	} {
		token := model.Token{
			Name: "video test", Key: "video" + tc.name + suffix, UserId: user.Id, Status: common.TokenStatusEnabled,
			ExpiredTime: -1, RemainQuota: user.Quota, Group: tc.group, AutoGroups: tc.autoGroups,
		}
		require.NoError(t, db.Create(&token).Error)
		h.tokens[tc.name] = videoCapabilityRouteToken{id: token.Id, bearer: "sk-" + token.Key}
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if r.Method == http.MethodGet && key == "example-narrow-backup" {
			h.mu.Lock()
			h.attempts = append(h.attempts, videoCapabilityAttempt{key: key, path: r.URL.Path})
			h.mu.Unlock()
			assert.Equal(t, "/v1/tasks/synthetic-video-task", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"code":200,"data":{"id":"synthetic-video-task","status":"completed","result":{"videos":[{"url":%q}]}}}`,
				"http://"+r.Host+"/fixtures/clip.mp4")
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/fixtures/clip.mp4" {
			assert.Empty(t, key, "artifact retrieval must not send an upstream credential")
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = io.WriteString(w, syntheticVideoBytes)
			return
		}
		var body map[string]any
		if err := common.DecodeJson(r.Body, &body); err != nil {
			t.Errorf("mock upstream got an invalid JSON request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
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
		if key == "muse-primary" || key == "example-narrow-primary" {
			if key == "muse-primary" && body["prompt"] == pauseMusePrimaryPrompt {
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
	pinned := h.tokens[""]
	pinned.bearer += "-" + fmt.Sprint(h.channels["muse-narrow"])
	h.tokens["pin"] = pinned
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
	return h.submitWithProtocol(t, path, token, "", body)
}

func (h *videoCapabilityRouteHarness) submitWithProtocol(t *testing.T, path, token, protocol string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	credential := h.tokens[token].bearer
	if strings.HasPrefix(path, "/pg/") {
		credential = h.browserToken
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	if protocol != "" {
		request.Header.Set("X-OmniMux-Video-Protocol", protocol)
	}
	recorder := httptest.NewRecorder()
	h.engine.ServeHTTP(recorder, request)
	return recorder
}

func (h *videoCapabilityRouteHarness) musePricing(t *testing.T) *model.EffectiveVideo {
	t.Helper()
	response := h.get(t, "/api/pricing", h.browserToken)
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

func (h *videoCapabilityRouteHarness) get(t *testing.T, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+bearer)
	response := httptest.NewRecorder()
	h.engine.ServeHTTP(response, request)
	return response
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

func (h *videoCapabilityRouteHarness) readBalances(path, token string) (videoCapabilityRouteBalances, error) {
	wallet, err := model.GetUserQuota(h.userID, true)
	balances := videoCapabilityRouteBalances{wallet: wallet}
	if err != nil || strings.HasPrefix(path, "/pg/") {
		return balances, err
	}
	storedToken, err := model.GetTokenByIds(h.tokens[token].id, h.userID)
	if err != nil {
		return balances, err
	}
	balances.tokenRemaining = storedToken.RemainQuota
	balances.tokenUsed = storedToken.UsedQuota
	return balances, nil
}

func (h *videoCapabilityRouteHarness) requireMismatch(t *testing.T, path, token string, request map[string]any) {
	t.Helper()
	attempts := h.attemptCount()
	before, err := h.readBalances(path, token)
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
	after, err := h.readBalances(path, token)
	require.NoError(t, err)
	assert.Equal(t, before, after, "rejected requests must leave wallet and token balances unchanged")
	var tasksAfter int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("user_id = ?", h.userID).Count(&tasksAfter).Error)
	assert.Equal(t, tasksBefore, tasksAfter)
}
