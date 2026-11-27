package router

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/relay"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
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

func TestFiveVideoGroups(t *testing.T) {
	h := newVideoCapabilityRouteHarness(t)
	// The shared harness also seeds default-group channels for other tests.
	// Isolate this catalog so its only memberships are the five fixtures below.
	require.NoError(t, model.DB.Where("model IN ?", []string{"muse-video", "seedance-2-5"}).Delete(&model.Ability{}).Error)
	bin := os.Getenv("OMNIMUX_TEST_GATEWAY_BINARY")
	rulesPath, err := filepath.Abs(filepath.Join("testdata", "issue-74", "five-group-rules.json"))
	require.NoError(t, err)
	raw, err := os.ReadFile(rulesPath)
	require.NoError(t, err)
	raw = bytes.ReplaceAll(raw, []byte("http://127.0.0.1:3211"), []byte(h.upstreamURL))

	const openAIPath, nativePath = "/v1/videos", "/v1/video/generations"
	groups := []struct {
		id, channelType                         int
		group, model, mapped, resolution, ratio string
		multiplier                              float64
		quota                                   int
	}{
		{1001, constant.ChannelTypeOpenAI, "pool", "muse-video", "muse-video", "", "16:9", 0.5, 2500},
		{1002, constant.ChannelTypeOpenAI, "seedance-2-5-480p", "seedance-2-5", "fixture-video-image-480p", "480p", "9:16", 0.25, 37500},
		{1003, constant.ChannelTypeOpenAI, "seedance-2-5-720p", "seedance-2-5", "fixture-video-720p", "720p", "9:16", 0.125, 18750},
		{1004, constant.ChannelTypeApiMart, "seedance-2-5-480p-pro", "seedance-2-5", "fixture-seedance-2.5-pro", "480p", "9:16", 0.75, 6672},
		{1005, constant.ChannelTypeApiMart, "seedance-2-5-720p-pro", "seedance-2-5", "fixture-seedance-2.5-pro", "720p", "16:9", 1.5, 30000},
	}
	pricing := map[string]float64{"default": 1, "fixture-hidden": 1}
	usable := map[string]string{"default": "Default"}
	for _, group := range groups {
		pricing[group.group] = group.multiplier
		usable[group.group] = group.group
		mapping, err := common.Marshal(map[string]string{group.model: group.mapped})
		require.NoError(t, err)
		channel := model.Channel{
			Id: group.id, Type: group.channelType, Key: group.group, Name: group.group,
			Models: group.model, Group: group.group, BaseURL: &h.upstreamURL, Status: common.ChannelStatusEnabled,
			Priority: common.GetPointer(int64(10)), AutoBan: common.GetPointer(0), ModelMapping: common.GetPointer(string(mapping)),
		}
		if group.id == 1002 {
			channel.Group += ",fixture-hidden"
		}
		require.NoError(t, model.DB.Create(&channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}

	// One extra physical channel in the same group forces a same-group retry.
	// Its variants deduplicate to the same public rule as the fallback channel.
	retryID := h.addSyntheticChannel(t, "fixture-economy-retry", "seedance-2-5", groups[1].group, groups[1].mapped, 1, 30)
	var ruleDocument struct {
		Models map[string][]common.RawMessage `json:"models"`
	}
	require.NoError(t, common.Unmarshal(raw, &ruleDocument))
	seedance := ruleDocument.Models["seedance-2-5"]
	retryRule := bytes.Replace(seedance[0], []byte(`"channel_id": 1002`), []byte(fmt.Sprintf(`"channel_id": %d`, retryID)), 1)
	require.NotEqual(t, seedance[0], common.RawMessage(retryRule))
	ruleDocument.Models["seedance-2-5"] = append(seedance, retryRule)
	raw, err = common.Marshal(ruleDocument)
	require.NoError(t, err)
	rules, err := modelcontracts.ReadVideoCapabilities(bytes.NewReader(raw))
	require.NoError(t, err)
	modelcontracts.SetVideoCapabilities(rules)
	model.InvalidatePricingCache()

	fetch := system_setting.GetFetchSetting()
	previousFetch := *fetch
	t.Cleanup(func() { *fetch = previousFetch })
	fetch.AllowPrivateIp = true
	mockURL, err := url.Parse(h.upstreamURL)
	require.NoError(t, err)
	fetch.AllowedPorts = []string{mockURL.Port()}
	groupRatios, err := common.Marshal(pricing)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(groupRatios)))
	usableGroups, err := common.Marshal(usable)
	require.NoError(t, err)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(string(usableGroups)))
	if bin != "" {
		file := filepath.Join(t.TempDir(), "rules.json")
		require.NoError(t, os.WriteFile(file, raw, 0o600))
		allowedPorts, err := common.Marshal(fetch.AllowedPorts)
		require.NoError(t, err)
		for key, value := range map[string]string{
			"GroupRatio": string(groupRatios), "UserUsableGroups": string(usableGroups),
			"ModelPrice": ratio_setting.ModelPrice2JSONString(), "RetryTimes": "1",
			"fetch_setting.allow_private_ip": "true", "fetch_setting.allowed_ports": string(allowedPorts),
			// The fixture exercises routing independently of the test host's load.
			"performance_setting.monitor_enabled": "false",
		} {
			require.NoError(t, model.DB.Create(&model.Option{Key: key, Value: value}).Error)
		}
		h.gateway = startFiveGroupGateway(t, bin, file)
	}

	var pricingRows []map[string]any
	catalog := make(map[string]any, 3)
	for _, name := range []string{"muse-video", "seedance-2-5"} {
		pricingResponse, contractResponse := h.syntheticCatalogResponses(t, name)
		row := pricingResponse["data"].([]map[string]any)[0]
		pricingRows = append(pricingRows, row)
		pricingResponse["data"] = pricingRows
		catalog["/api/pricing"] = pricingResponse
		catalog["/api/model_contract/"+name] = contractResponse
		data := contractResponse["data"].(map[string]any)
		view := data["effective_video"].(map[string]any)
		assert.Equal(t, view, row["effective_video"])
		assert.NotEmpty(t, view["version"])
		published := view["groups"].(map[string]any)
		assert.NotContains(t, published, "fixture-hidden")
		var wantGroups []string
		for _, group := range groups {
			if group.model != name {
				continue
			}
			wantGroups = append(wantGroups, group.group)
			variants := published[group.group].([]any)
			if name == "muse-video" {
				assert.Len(t, variants, 3)
				assert.Equal(t, "openai_video", data["contract"].(map[string]any)["gateway_protocol"])
				continue
			}
			require.Len(t, variants, 1)
			variant := variants[0].(map[string]any)
			assert.Equal(t, group.resolution, videoFixtureParameter(variant, "resolution")["fixed"])
			assert.Equal(t, group.ratio, videoFixtureParameter(variant, "aspect_ratio")["fixed"])
			if group.channelType == constant.ChannelTypeApiMart {
				assert.Equal(t, "video_generations", variant["gateway_protocol"])
				assert.Equal(t, "text_to_video", variant["id"])
				assert.EqualValues(t, 4, videoFixtureParameter(variant, "duration")["fixed"])
				assert.Len(t, variant["inputs"], 1)
				continue
			}
			assert.Equal(t, "openai_video", variant["gateway_protocol"])
			assert.Equal(t, "video_multi_ref", variant["id"])
			assert.Equal(t, "image_reference", variant["modeKind"])
			assert.EqualValues(t, 30, videoFixtureParameter(variant, "duration")["fixed"])
			resolution := videoFixtureParameter(variant, "resolution")
			assert.Equal(t, group.resolution, resolution["default"])
			assert.NotContains(t, resolution, "required")
			inputs := variant["inputs"].([]any)
			require.Len(t, inputs, 2)
			image := inputs[1].(map[string]any)
			assert.Equal(t, "reference_images", image["slot"])
			assert.EqualValues(t, 1, image["min"])
			assert.EqualValues(t, 1, image["max"])
			assert.NotContains(t, image, "firstItemRole")
			assert.NotContains(t, image, "allowedMimes")
			assert.NotContains(t, image, "maxSizeMb")
			assert.NotContains(t, variant["output"], "formats")
		}
		assert.ElementsMatch(t, wantGroups, slices.Collect(maps.Keys(published)))
		encoded, err := common.Marshal(view)
		require.NoError(t, err)
		for _, private := range []string{"fixture-video-", "fixture-seedance-", h.upstreamURL, "Synthetic", "1002"} {
			assert.NotContains(t, string(encoded), private)
		}
	}

	originalResolver := service.GetTaskAdaptorFunc
	t.Cleanup(func() { service.GetTaskAdaptorFunc = originalResolver })
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		return relay.GetTaskAdaptor(platform)
	}
	const imageURL = "https://media.example/reference.png"
	for _, group := range groups {
		t.Run(group.group, func(t *testing.T) {
			path, protocol, operation, duration := openAIPath, "", "text_to_video", 1
			if group.channelType == constant.ChannelTypeApiMart {
				path, duration = nativePath, 4
				if group.resolution == "480p" {
					protocol = "video_generations"
				}
			} else if group.model == "seedance-2-5" {
				operation, duration = "video_multi_ref", 30
			}
			body := map[string]any{
				"model": group.model + "@" + group.group, "prompt": "Animate the fixture", "operation": operation,
				"duration": duration, "aspect_ratio": group.ratio,
			}
			if group.resolution != "" && group.channelType == constant.ChannelTypeApiMart {
				body["resolution"] = group.resolution
			}
			if operation == "video_multi_ref" {
				body["images"] = []string{imageURL}
				if group.id == 1002 {
					// The legacy shared path has neither an operation nor a protocol
					// header. The explicit group determines its decoder and resolution.
					path = nativePath
					delete(body, "operation")
					delete(body, "duration")
					body["seconds"] = "30"
				}
			}
			start := h.attemptCount()
			before, err := h.readBalances(path, "")
			require.NoError(t, err)
			rejected := maps.Clone(body)
			rejected["duration"] = duration + 1
			if group.model == "muse-video" {
				rejected["images"] = []string{imageURL}
			}
			response := h.submitWithProtocol(t, path, "", protocol, rejected)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			if operation == "video_multi_ref" {
				shorter := maps.Clone(body)
				delete(shorter, "seconds")
				shorter["duration"] = 4
				h.requireMismatch(t, path, "", shorter)
				for field, value := range map[string]any{
					"images": []string{imageURL, imageURL}, "image_urls": []string{imageURL},
					"image_with_roles": []map[string]string{{"role": "first_frame", "url": imageURL}},
					"resolution":       map[string]string{"480p": "720p", "720p": "480p"}[group.resolution],
					"size":             "16:9", "generate_audio": false,
				} {
					invalid := maps.Clone(body)
					invalid[field] = value
					response := h.submit(t, path, "", invalid)
					assert.Equal(t, http.StatusBadRequest, response.Code, "%s: %s", field, response.Body.String())
				}
			}
			if group.id == 1002 {
				unauthorized := maps.Clone(body)
				unauthorized["model"] = group.model + "@fixture-hidden"
				response := h.submit(t, path, "", unauthorized)
				assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
			}
			assert.Equal(t, start, h.attemptCount())
			uncharged, err := h.readBalances(path, "")
			require.NoError(t, err)
			assert.Equal(t, before, uncharged)

			response = h.submitWithProtocol(t, path, "", protocol, body)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			attempts := h.attemptsFrom(start)
			keys := []string{group.group}
			if group.id == 1002 {
				keys = append([]string{"fixture-economy-retry"}, keys...)
			}
			require.Len(t, attempts, len(keys))
			for i, attempt := range attempts {
				assert.Equal(t, keys[i], attempt.key)
				assert.Equal(t, group.mapped, attempt.body["model"])
				assert.NotContains(t, attempt.body, "operation")
				if operation == "video_multi_ref" {
					assert.Equal(t, openAIPath, attempt.path)
					assert.Equal(t, []any{imageURL}, attempt.body["images"])
					assert.Equal(t, group.resolution, attempt.body["resolution"])
					assert.Equal(t, group.ratio, attempt.body["aspect_ratio"])
					assert.EqualValues(t, duration, attempt.body["seconds"])
					assert.NotContains(t, attempt.body, "duration")
				} else {
					assert.EqualValues(t, duration, attempt.body["duration"])
					assert.NotContains(t, attempt.body, "images")
					if group.channelType == constant.ChannelTypeApiMart {
						assert.Equal(t, openAIPath+"/generations", attempt.path)
						assert.Equal(t, group.resolution, attempt.body["resolution"])
						assert.Equal(t, group.ratio, attempt.body["size"])
					}
				}
			}
			task := h.latestTask(t)
			assert.Equal(t, group.id, task.ChannelId)
			assert.Equal(t, group.group, task.Group)
			if group.channelType != constant.ChannelTypeApiMart {
				assert.Equal(t, operation, task.Action)
			}
			assert.Equal(t, group.quota, task.Quota)
			require.NotNil(t, task.PrivateData.BillingContext)
			assert.Equal(t, group.multiplier, task.PrivateData.BillingContext.GroupRatio)
			assert.EqualValues(t, duration, task.PrivateData.BillingContext.OtherRatios["seconds"])

			upstreamID := task.GetUpstreamTaskID()
			pollStart := h.attemptCount()
			require.NoError(t, service.UpdateVideoTasks(t.Context(), task.Platform, map[int][]string{task.ChannelId: {upstreamID}}, map[string]*model.Task{upstreamID: &task}))
			polls := h.attemptsFrom(pollStart)
			require.Len(t, polls, 1)
			assert.Equal(t, group.group, polls[0].key)
			queryPath := openAIPath
			upstreamQuery := openAIPath
			if group.channelType == constant.ChannelTypeApiMart {
				queryPath = nativePath
				upstreamQuery = "/v1/tasks"
			}
			assert.Equal(t, upstreamQuery+"/"+upstreamID, polls[0].path)
			query := h.get(t, queryPath+"/"+task.TaskID, h.tokens[""].bearer)
			require.Equal(t, http.StatusOK, query.Code, query.Body.String())
			if queryPath == nativePath {
				var result dto.TaskResponse[dto.TaskDto]
				require.NoError(t, common.Unmarshal(query.Body.Bytes(), &result))
				assert.True(t, result.IsSuccess())
				assert.Equal(t, task.TaskID, result.Data.TaskID)
				assert.Equal(t, string(model.TaskStatusSuccess), result.Data.Status)
			} else {
				var result kitdto.OpenAIVideo
				require.NoError(t, common.Unmarshal(query.Body.Bytes(), &result))
				assert.Equal(t, task.TaskID, result.ID)
				assert.Equal(t, kitdto.VideoStatusCompleted, result.Status)
				assert.Equal(t, group.model, result.Model)
			}
			assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), h.latestTask(t).Status)
			clip := h.get(t, openAIPath+"/"+task.TaskID+"/content", h.tokens[""].bearer)
			require.Equal(t, http.StatusOK, clip.Code, clip.Body.String())
			assert.Equal(t, "video/mp4", clip.Header().Get("Content-Type"))
			assert.Equal(t, syntheticVideoBytes, clip.Body.String())
			settled, err := h.readBalances(path, "")
			require.NoError(t, err)
			assert.Equal(t, group.quota, before.wallet-settled.wallet)
			assert.Equal(t, group.quota, before.tokenRemaining-settled.tokenRemaining)
			assert.Equal(t, group.quota, settled.tokenUsed-before.tokenUsed)
		})
	}
	if h.gateway == "" {
		t.Run("economy expression reserves and settles reported usage", func(t *testing.T) {
			h.assertEconomyExpressionSettlement(t, nativePath, groups[1].group, imageURL)
		})
		t.Run("economy group with two resolutions requires an explicit choice", func(t *testing.T) {
			h.assertEconomyGroupResolutionRequired(t, nativePath, groups[1].group, groups[2].group, groups[2].id, imageURL)
		})
	}
	if path := os.Getenv("OMNIMUX_TEST_FIVE_GROUP_CATALOG"); path != "" && !t.Failed() {
		raw, err := common.Marshal(catalog)
		require.NoError(t, err)
		formatted, err := common.IndentJson(raw)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, append(formatted, '\n'), 0o600))
	}
}

func (h *videoCapabilityRouteHarness) assertEconomyExpressionSettlement(t *testing.T, path, group, imageURL string) {
	t.Helper()
	const expression = `u("size") == "720x1280" ? tier("reported_size", u("seconds") * 0.002) : tier("duration", u("seconds") * 0.001)`
	previous, err := common.Marshal(billing_setting.GetPluginBillingExprCopy())
	require.NoError(t, err)
	loadExpression := func(raw []byte) {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{billing_setting.PluginBillingExprOption: string(raw)}))
	}
	t.Cleanup(func() { loadExpression(previous) })
	configured, err := common.Marshal(map[string]string{billing_setting.PluginBillingExprKey("sora", "seedance-2-5"): expression})
	require.NoError(t, err)
	loadExpression(configured)

	before, err := h.readBalances(path, "")
	require.NoError(t, err)
	response := h.submit(t, path, "", map[string]any{
		"model": "seedance-2-5@" + group, "prompt": "Animate the fixture", "duration": 30, "aspect_ratio": "9:16", "images": []string{imageURL},
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	task := h.latestTask(t)
	require.NotNil(t, task.PrivateData.BillingContext)
	snapshot := task.PrivateData.BillingContext.TieredSnapshot
	require.NotNil(t, snapshot)
	assert.True(t, snapshot.TaskUsageBilling)
	assert.Equal(t, expression, snapshot.ExprString)
	assert.Equal(t, map[string]any{"seconds": float64(30)}, snapshot.UsageFacts)
	// The fixture group ratio is 0.25. Submission reserves 30 seconds at the
	// duration tier. The mock reports 28 seconds on completion without a size.
	const reservedQuota, settledQuota = 3750, 3500
	assert.Equal(t, reservedQuota, snapshot.EstimatedQuotaAfterGroup)
	assert.Equal(t, reservedQuota, task.Quota)
	reserved, err := h.readBalances(path, "")
	require.NoError(t, err)
	assert.Equal(t, reservedQuota, before.wallet-reserved.wallet)
	assert.Equal(t, reservedQuota, before.tokenRemaining-reserved.tokenRemaining)

	// Poll from the persisted task after the capability file has been withdrawn.
	rules := modelcontracts.SetVideoCapabilities(nil)
	t.Cleanup(func() { modelcontracts.SetVideoCapabilities(rules) })
	upstreamID := task.GetUpstreamTaskID()
	require.NoError(t, service.UpdateVideoTasks(t.Context(), task.Platform, map[int][]string{task.ChannelId: {upstreamID}}, map[string]*model.Task{upstreamID: &task}))
	completed := h.latestTask(t)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), completed.Status)
	assert.Equal(t, settledQuota, completed.Quota)
	settled, err := h.readBalances(path, "")
	require.NoError(t, err)
	assert.Equal(t, settledQuota, before.wallet-settled.wallet)
	assert.Equal(t, settledQuota, before.tokenRemaining-settled.tokenRemaining)
	assert.Equal(t, settledQuota, settled.tokenUsed-before.tokenUsed)
}

func (h *videoCapabilityRouteHarness) assertEconomyGroupResolutionRequired(t *testing.T, path, mixedGroup, singleGroup string, secondChannelID int, imageURL string) {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, secondChannelID).Error)
	channel.Group += "," + mixedGroup
	require.NoError(t, channel.AddAbilities(nil))
	model.InvalidatePricingCache()

	pricing, contract := h.syntheticCatalogResponses(t, "seedance-2-5")
	view := contract["data"].(map[string]any)["effective_video"].(map[string]any)
	assert.Equal(t, view, pricing["data"].([]map[string]any)[0]["effective_video"])
	groups := view["groups"].(map[string]any)
	variants := groups[mixedGroup].([]any)
	require.Len(t, variants, 2)
	var observed []any
	for _, value := range variants {
		resolution := videoFixtureParameter(value.(map[string]any), "resolution")
		observed = append(observed, resolution["fixed"])
		assert.Equal(t, true, resolution["required"])
		assert.NotContains(t, resolution, "default")
	}
	assert.ElementsMatch(t, []string{"480p", "720p"}, observed)
	remaining := groups[singleGroup].([]any)
	require.Len(t, remaining, 1)
	resolution := videoFixtureParameter(remaining[0].(map[string]any), "resolution")
	assert.Equal(t, "720p", resolution["default"])
	assert.NotContains(t, resolution, "required")

	body := map[string]any{
		"model": "seedance-2-5@" + mixedGroup, "prompt": "Animate the fixture", "duration": 30, "aspect_ratio": "9:16", "images": []string{imageURL},
	}
	start := h.attemptCount()
	before, err := h.readBalances(path, "")
	require.NoError(t, err)
	rejected := h.submit(t, path, "", body)
	require.Equal(t, http.StatusBadRequest, rejected.Code, rejected.Body.String())
	assert.Equal(t, start, h.attemptCount())
	uncharged, err := h.readBalances(path, "")
	require.NoError(t, err)
	assert.Equal(t, before, uncharged)
	body["resolution"] = "720p"
	accepted := h.submit(t, path, "", body)
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	attempts := h.attemptsFrom(start)
	require.Len(t, attempts, 1)
	assert.Equal(t, singleGroup, attempts[0].key)
	assert.Equal(t, "720p", attempts[0].body["resolution"])
	task := h.latestTask(t)
	assert.Equal(t, secondChannelID, task.ChannelId)
	assert.Equal(t, mixedGroup, task.Group)
}

func startFiveGroupGateway(t *testing.T, binary, rules string) string {
	t.Helper()
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	dir := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	require.NoError(t, listener.Close())
	log, err := os.Create(filepath.Join(dir, "gateway.log"))
	require.NoError(t, err)
	defer log.Close()
	cmd := exec.Command(binary, "-log-dir", dir)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=/usr/bin:/bin", "HOME=" + dir, "TMPDIR=" + dir, "LISTEN_HOST=127.0.0.1", "PORT=" + port,
		"SQLITE_PATH=" + common.SQLitePath, "SESSION_SECRET=" + common.SessionSecret,
		"OMNIMUX_VIDEO_CAPABILITIES_FILE=" + rules, "UPDATE_TASK=false", "SHUTDOWN_TIMEOUT_SECONDS=2",
	}
	cmd.Stdout, cmd.Stderr = log, log
	require.NoError(t, cmd.Start())
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	t.Cleanup(func() {
		signalErr := cmd.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-finished:
			if signalErr != nil {
				t.Errorf("could not send SIGTERM to local gateway: %v (wait: %v)", signalErr, err)
				break
			}
			// The HTTP listener can serve requests before the signal handler is installed.
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signal() == syscall.SIGTERM {
					break
				}
			}
			assert.NoError(t, err, "wait for local gateway shutdown")
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-finished
			t.Error("local gateway did not shut down")
		}
		if t.Failed() {
			output, err := os.ReadFile(log.Name())
			if err == nil {
				t.Logf("local gateway log:\n%s", output)
			}
		}
	})
	address := "http://127.0.0.1:" + port
	client := &http.Client{Timeout: time.Second}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := client.Get(address + "/api/status")
		if assert.NoError(c, err) {
			resp.Body.Close()
			assert.Equal(c, http.StatusOK, resp.StatusCode)
		}
	}, 15*time.Second, 50*time.Millisecond)
	return address
}

func videoFixtureParameter(variant map[string]any, name string) map[string]any {
	for _, value := range variant["parameters"].([]any) {
		parameter := value.(map[string]any)
		if parameter["name"] == name {
			return parameter
		}
	}
	return nil
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
			// These capability fixtures have no display configuration. Verify
			// the fallback before normalizing the legacy order for comparison.
			groups := row["enable_groups"].([]any)
			require.Equal(t, groups, row["group_order"])
			slices.SortFunc(groups, func(a, b any) int { return strings.Compare(a.(string), b.(string)) })
			row["group_order"] = groups
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
	gateway      string
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
		if r.Method == http.MethodGet && (key == "pool" || strings.HasPrefix(key, "seedance-2-5-")) {
			h.mu.Lock()
			h.attempts = append(h.attempts, videoCapabilityAttempt{key: key, path: r.URL.Path})
			h.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			clip := "http://" + r.Host + "/fixtures/clip.mp4"
			if strings.HasSuffix(key, "-pro") {
				_, _ = fmt.Fprintf(w, `{"code":200,"data":{"status":"completed","result":{"videos":[{"url":%q}]}}}`, clip)
			} else if key == "seedance-2-5-480p" {
				_, _ = fmt.Fprintf(w, `{"status":"completed","video_url":%q,"seconds":"28"}`, clip)
			} else {
				_, _ = fmt.Fprintf(w, `{"status":"completed","video_url":%q}`, clip)
			}
			return
		}
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
		if key == "muse-primary" || key == "example-narrow-primary" || key == "fixture-economy-retry" {
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
	return h.serve(t, request)
}

func (h *videoCapabilityRouteHarness) serve(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	if h.gateway == "" {
		h.engine.ServeHTTP(recorder, request)
		return recorder
	}
	outgoing, err := http.NewRequestWithContext(t.Context(), request.Method, h.gateway+request.URL.RequestURI(), request.Body)
	require.NoError(t, err)
	outgoing.Header = request.Header
	outgoing.ContentLength = request.ContentLength
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(outgoing)
	require.NoError(t, err)
	defer response.Body.Close()
	maps.Copy(recorder.Header(), response.Header)
	recorder.WriteHeader(response.StatusCode)
	_, err = io.Copy(recorder, response.Body)
	require.NoError(t, err)
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
	return h.serve(t, request)
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

func TestApiMartVideoTierBillingPolicyAndAutoDurationSettlement(t *testing.T) {
	t.Run("channel save validation enforces video_tier_billing_policy", func(t *testing.T) {
		for _, raw := range []string{"", kitdto.VideoTierBillingPolicyPreset, kitdto.VideoTierBillingPolicyNone} {
			ch := &model.Channel{Type: constant.ChannelTypeApiMart}
			ch.SetOtherSettings(kitdto.ChannelOtherSettings{VideoTierBillingPolicy: raw})
			require.NoError(t, ch.ValidateSettings(), raw)
		}
		for _, invalid := range []string{"custom", "flat", "NONE", " "} {
			ch := &model.Channel{Type: constant.ChannelTypeApiMart}
			ch.SetOtherSettings(kitdto.ChannelOtherSettings{VideoTierBillingPolicy: invalid})
			err := ch.ValidateSettings()
			require.Error(t, err, invalid)
			assert.Contains(t, err.Error(), "video_tier_billing_policy")
		}
	})

	h := newVideoCapabilityRouteHarness(t)
	// Disable managed video rules for this test so phase-one apimart routing
	// exercises seedance-2-5 directly across 480p, 720p, 1080p, and duration=-1.
	modelcontracts.SetVideoCapabilities(nil)
	require.NoError(t, model.DB.Where("model = ?", "seedance-2-5").Delete(&model.Ability{}).Error)

	var (
		pollMu   sync.Mutex
		pollBody = `{"code":200,"data":{"id":"auto-task","status":"completed","result":{"videos":[{"url":"http://127.0.0.1/clip.mp4","duration":8}]}}}`
		nextID   int
	)
	setPollBody := func(body string) {
		pollMu.Lock()
		pollBody = body
		pollMu.Unlock()
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			pollMu.Lock()
			body := pollBody
			pollMu.Unlock()
			_, _ = io.WriteString(w, body)
			return
		}
		var req map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &req))
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		h.mu.Lock()
		h.attempts = append(h.attempts, videoCapabilityAttempt{key: key, path: r.URL.Path, body: req})
		nextID++
		taskID := fmt.Sprintf("auto-task-%d", nextID)
		h.mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"code":200,"data":[{"status":"submitted","task_id":%q}]}`, taskID)
	}))
	t.Cleanup(upstream.Close)

	originalResolver := service.GetTaskAdaptorFunc
	t.Cleanup(func() { service.GetTaskAdaptorFunc = originalResolver })
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		return relay.GetTaskAdaptor(platform)
	}
	// Keep async perf_metrics Redis sampling off so t.Cleanup restoring RedisEnabled stays race-free under -race.
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": "false"}))
	t.Cleanup(func() {
		assert.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": "true"}))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-2-5":0.01}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"tier-preset":0.75,"tier-none":0.75,"tier-invalid":0.75,"tier-free":0}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","tier-preset":"Preset","tier-none":"None","tier-invalid":"Invalid","tier-free":"Free"}`))

	addChannel := func(group, policy string) int {
		ch := model.Channel{
			Type:     constant.ChannelTypeApiMart,
			Key:      group,
			Name:     group,
			Models:   "seedance-2-5",
			Group:    group,
			BaseURL:  &upstream.URL,
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer(int64(10)),
			AutoBan:  common.GetPointer(0),
		}
		if policy != "" {
			ch.SetOtherSettings(kitdto.ChannelOtherSettings{VideoTierBillingPolicy: policy})
		}
		require.NoError(t, model.DB.Create(&ch).Error)
		require.NoError(t, ch.AddAbilities(nil))
		return ch.Id
	}

	presetChannelID := addChannel("tier-preset", kitdto.VideoTierBillingPolicyPreset)
	noneChannelID := addChannel("tier-none", kitdto.VideoTierBillingPolicyNone)
	addChannel("tier-invalid", "bogus")
	addChannel("tier-free", kitdto.VideoTierBillingPolicyNone)

	const submitPath = "/v1/video/generations"
	const imageURL = "https://cdn.example.com/ref.png"

	t.Run("preset and none channels isolate tier multipliers without double discount", func(t *testing.T) {
		// ModelPrice=0.01 => 5000 quota/s; GroupRatio=0.75 => 3750 quota/s; duration=4 => base 15000.
		// Preset 480p with reference image applies 0.0576/0.216 => 15000 * 0.0576 / 0.216 = 4000.
		presetResp := h.submit(t, submitPath, "", map[string]any{
			"model":      "seedance-2-5@tier-preset",
			"operation":  "first_frame",
			"prompt":     "animate",
			"duration":   4,
			"resolution": "480p",
			"image_with_roles": []map[string]string{
				{"url": imageURL, "role": "first_frame"},
			},
		})
		require.Equal(t, http.StatusOK, presetResp.Code, presetResp.Body.String())
		presetTask := h.latestTask(t)
		assert.Equal(t, presetChannelID, presetTask.ChannelId)
		assert.Equal(t, 4000, presetTask.Quota)
		require.NotNil(t, presetTask.PrivateData.BillingContext)
		assert.False(t, presetTask.PrivateData.BillingContext.AutoDurationBilling)
		assert.InDelta(t, 0.0576/0.216, presetTask.PrivateData.BillingContext.OtherRatios["tier"], 1e-9)

		// None policy disables built-in tier table => 3750 * 4 = 15000, no tier key.
		noneResp := h.submit(t, submitPath, "", map[string]any{
			"model":      "seedance-2-5@tier-none",
			"operation":  "first_frame",
			"prompt":     "animate",
			"duration":   4,
			"resolution": "480p",
			"image_with_roles": []map[string]string{
				{"url": imageURL, "role": "first_frame"},
			},
		})
		require.Equal(t, http.StatusOK, noneResp.Code, noneResp.Body.String())
		noneTask := h.latestTask(t)
		assert.Equal(t, noneChannelID, noneTask.ChannelId)
		assert.Equal(t, 15000, noneTask.Quota)
		require.NotNil(t, noneTask.PrivateData.BillingContext)
		assert.False(t, noneTask.PrivateData.BillingContext.AutoDurationBilling)
		assert.Equal(t, map[string]float64{"seconds": 4}, noneTask.PrivateData.BillingContext.OtherRatios)
	})

	t.Run("seedance-2-5 1080p allowed under none and rejected under preset or invalid policy", func(t *testing.T) {
		beforeAttempts := h.attemptCount()
		beforeBalances, err := h.readBalances(submitPath, "")
		require.NoError(t, err)

		preset1080 := h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-preset", "operation": "text_to_video",
			"prompt": "city", "duration": 4, "resolution": "1080p",
		})
		assert.Equal(t, http.StatusBadRequest, preset1080.Code, preset1080.Body.String())

		invalidPolicyResp := h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-invalid", "operation": "text_to_video",
			"prompt": "city", "duration": 4, "resolution": "720p",
		})
		assert.Equal(t, http.StatusBadRequest, invalidPolicyResp.Code, invalidPolicyResp.Body.String())

		assert.Equal(t, beforeAttempts, h.attemptCount(), "rejected billing policy/tier must never hit upstream")
		afterReject, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, beforeBalances, afterReject)

		none1080 := h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-none", "operation": "text_to_video",
			"prompt": "city", "duration": 4, "resolution": "1080p",
		})
		require.Equal(t, http.StatusOK, none1080.Code, none1080.Body.String())
		assert.Equal(t, 15000, h.latestTask(t).Quota)
	})

	t.Run("duration -1 ModelPrice path settles actual duration, enforces cap, refunds failures, and stays idempotent", func(t *testing.T) {
		// 1) Actual 8s < 30s pre-charge (112500 -> 30000, refund 82500)
		before, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		resp := h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-none", "operation": "text_to_video",
			"prompt": "auto length", "duration": -1, "resolution": "1080p",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		task := h.latestTask(t)
		require.NotNil(t, task.PrivateData.BillingContext)
		assert.True(t, task.PrivateData.BillingContext.PerCallBilling)
		assert.True(t, task.PrivateData.BillingContext.AutoDurationBilling)
		assert.Equal(t, 112500, task.Quota)

		upstreamID := task.GetUpstreamTaskID()
		setPollBody(fmt.Sprintf(`{"code":200,"data":{"id":%q,"status":"completed","result":{"videos":[{"url":"%s/clip.mp4","duration":8}]}}}`, upstreamID, upstream.URL))
		require.NoError(t, service.UpdateVideoTasks(t.Context(), task.Platform, map[int][]string{task.ChannelId: {upstreamID}}, map[string]*model.Task{upstreamID: &task}))

		completed := h.latestTask(t)
		assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), completed.Status)
		assert.Equal(t, 30000, completed.Quota)
		settled, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, 30000, before.wallet-settled.wallet)
		assert.Equal(t, 30000, before.tokenRemaining-settled.tokenRemaining)
		assert.Equal(t, 30000, settled.tokenUsed-before.tokenUsed)

		// Replay poll must be a CAS no-op (no duplicate refund).
		stale := task
		stale.Status = model.TaskStatusSubmitted
		stale.Quota = 112500
		require.NoError(t, service.UpdateVideoTasks(t.Context(), stale.Platform, map[int][]string{stale.ChannelId: {upstreamID}}, map[string]*model.Task{upstreamID: &stale}))
		afterReplay, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, settled, afterReplay)

		// 2) Upstream reports duration exceeding pre-charge (45s > 30s) => keeps 30s hold (no overcharge)
		beforeOver, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		resp = h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-none", "operation": "text_to_video",
			"prompt": "over duration", "duration": -1, "resolution": "720p",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		overTask := h.latestTask(t)
		overUpstreamID := overTask.GetUpstreamTaskID()
		setPollBody(fmt.Sprintf(`{"code":200,"data":{"id":%q,"status":"completed","result":{"videos":[{"url":"%s/clip.mp4","duration":45}]}}}`, overUpstreamID, upstream.URL))
		require.NoError(t, service.UpdateVideoTasks(t.Context(), overTask.Platform, map[int][]string{overTask.ChannelId: {overUpstreamID}}, map[string]*model.Task{overUpstreamID: &overTask}))
		assert.Equal(t, 112500, h.latestTask(t).Quota)
		afterOver, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, 112500, beforeOver.wallet-afterOver.wallet)

		// 3) Upstream fails => full refund
		beforeFail, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		resp = h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-none", "operation": "text_to_video",
			"prompt": "will fail", "duration": -1, "resolution": "720p",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		failTask := h.latestTask(t)
		failUpstreamID := failTask.GetUpstreamTaskID()
		setPollBody(fmt.Sprintf(`{"code":200,"data":{"id":%q,"status":"failed","error":{"message":"render failed"}}}`, failUpstreamID))
		require.NoError(t, service.UpdateVideoTasks(t.Context(), failTask.Platform, map[int][]string{failTask.ChannelId: {failUpstreamID}}, map[string]*model.Task{failUpstreamID: &failTask}))
		assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), h.latestTask(t).Status)
		assert.Equal(t, 0, h.latestTask(t).Quota)
		afterFail, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, beforeFail, afterFail)

		// 4) Free group (GroupRatio=0) => stays 0 charge
		beforeFree, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		resp = h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-free", "operation": "text_to_video",
			"prompt": "free run", "duration": -1, "resolution": "720p",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		freeTask := h.latestTask(t)
		assert.Equal(t, 0, freeTask.Quota)
		freeUpstreamID := freeTask.GetUpstreamTaskID()
		setPollBody(fmt.Sprintf(`{"code":200,"data":{"id":%q,"status":"completed","result":{"videos":[{"url":"%s/clip.mp4","duration":8}]}}}`, freeUpstreamID, upstream.URL))
		require.NoError(t, service.UpdateVideoTasks(t.Context(), freeTask.Platform, map[int][]string{freeTask.ChannelId: {freeUpstreamID}}, map[string]*model.Task{freeUpstreamID: &freeTask}))
		assert.Equal(t, 0, h.latestTask(t).Quota)
		afterFree, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, beforeFree, afterFree)
	})

	t.Run("duration -1 TieredSnapshot path settles actual duration and caps oversized reports", func(t *testing.T) {
		prevModes, err := common.Marshal(billing_setting.GetBillingModeCopy())
		require.NoError(t, err)
		prevExprs, err := common.Marshal(billing_setting.GetBillingExprCopy())
		require.NoError(t, err)
		loadBilling := func(modes, exprs []byte) {
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
				"billing_setting.billing_mode": string(modes),
				"billing_setting.billing_expr": string(exprs),
			}))
		}
		t.Cleanup(func() { loadBilling(prevModes, prevExprs) })

		modesJSON, err := common.Marshal(map[string]string{"seedance-2-5": billing_setting.BillingModeTieredExpr})
		require.NoError(t, err)
		exprsJSON, err := common.Marshal(map[string]string{"seedance-2-5": `tier("duration", u("seconds") * 0.01)`})
		require.NoError(t, err)
		loadBilling(modesJSON, exprsJSON)

		// 1) Actual 8s < 30s pre-charge (112500 -> 30000)
		before, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		resp := h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-none", "operation": "text_to_video",
			"prompt": "tiered auto", "duration": -1, "resolution": "1080p",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		task := h.latestTask(t)
		require.NotNil(t, task.PrivateData.BillingContext)
		require.NotNil(t, task.PrivateData.BillingContext.TieredSnapshot)
		assert.True(t, task.PrivateData.BillingContext.AutoDurationBilling)
		assert.Equal(t, 112500, task.Quota)

		upstreamID := task.GetUpstreamTaskID()
		setPollBody(fmt.Sprintf(`{"code":200,"data":{"id":%q,"status":"completed","result":{"videos":[{"url":"%s/clip.mp4","duration":8}]}}}`, upstreamID, upstream.URL))
		require.NoError(t, service.UpdateVideoTasks(t.Context(), task.Platform, map[int][]string{task.ChannelId: {upstreamID}}, map[string]*model.Task{upstreamID: &task}))

		completed := h.latestTask(t)
		assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), completed.Status)
		assert.Equal(t, 30000, completed.Quota)
		settled, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, 30000, before.wallet-settled.wallet)
		assert.Equal(t, 30000, before.tokenRemaining-settled.tokenRemaining)

		// 2) Upstream reports 45s (> 30s pre-charge) => keeps 30s pre-charge
		beforeOver, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		resp = h.submit(t, submitPath, "", map[string]any{
			"model": "seedance-2-5@tier-none", "operation": "text_to_video",
			"prompt": "tiered over", "duration": -1, "resolution": "1080p",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		overTask := h.latestTask(t)
		overUpstreamID := overTask.GetUpstreamTaskID()
		setPollBody(fmt.Sprintf(`{"code":200,"data":{"id":%q,"status":"completed","result":{"videos":[{"url":"%s/clip.mp4","duration":45}]}}}`, overUpstreamID, upstream.URL))
		require.NoError(t, service.UpdateVideoTasks(t.Context(), overTask.Platform, map[int][]string{overTask.ChannelId: {overUpstreamID}}, map[string]*model.Task{overUpstreamID: &overTask}))
		assert.Equal(t, 112500, h.latestTask(t).Quota)
		afterOver, err := h.readBalances(submitPath, "")
		require.NoError(t, err)
		assert.Equal(t, 112500, beforeOver.wallet-afterOver.wallet)
	})
}

func TestSeedance25AndFlashMultiGroupCapabilities(t *testing.T) {
	h := newVideoCapabilityRouteHarness(t)
	require.NoError(t, model.DB.Where("model IN ?", []string{"seedance-2-5", "seedance-2-5-flash"}).Delete(&model.Ability{}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"muse-video":0.01,"seedance-2-5":0.283,"seedance-2-5-flash":0.145588}`))

	groupRatioMap := map[string]float64{
		"default":                     1,
		"seedance-2-5-480p-eco":       0.2,
		"seedance-2-5-720p":           0.55,
		"seedance-2-5-1080p-plus":     1.05,
		"seedance-2-5-flash-480p":     0.75,
		"seedance-2-5-flash-480p-eco": 0.25,
		"seedance-2-5-flash-720p-eco": 0.4,
	}
	usableGroupMap := map[string]string{"default": "Default"}
	for groupName := range groupRatioMap {
		if groupName != "default" {
			usableGroupMap[groupName] = groupName
		}
	}
	groupRatiosJSON, err := common.Marshal(groupRatioMap)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(groupRatiosJSON)))
	usableGroupsJSON, err := common.Marshal(usableGroupMap)
	require.NoError(t, err)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(string(usableGroupsJSON)))

	ch480Eco := h.addSyntheticChannel(t, "s25-480p-eco", "seedance-2-5", "seedance-2-5-480p-eco", "BT-2.5-480p", constant.ChannelTypeOpenAI, 10)
	ch720Std := h.addSyntheticChannel(t, "s25-720p", "seedance-2-5", "seedance-2-5-720p", "sd-2.5-720p-pro", constant.ChannelTypeOpenAI, 10)
	ch1080Plus := h.addSyntheticChannel(t, "s25-1080p-plus", "seedance-2-5", "seedance-2-5-1080p-plus", "sd-2.5-1080p-plus", constant.ChannelTypeOpenAI, 10)
	chFlashNative := h.addSyntheticChannel(t, "flash-480p-native", "seedance-2-5-flash", "seedance-2-5-flash-480p", "seedance-2-5-flash", constant.ChannelTypeApiMart, 10)
	chFlash480Eco := h.addSyntheticChannel(t, "flash-480p-eco", "seedance-2-5-flash", "seedance-2-5-flash-480p-eco", "jiuyue-sd25-flash-480p", constant.ChannelTypeOpenAI, 10)
	chFlash720Eco := h.addSyntheticChannel(t, "flash-720p-eco", "seedance-2-5-flash", "seedance-2-5-flash-720p-eco", "jiuyue-sd25-flash-720p", constant.ChannelTypeOpenAI, 10)

	seedanceOpenAIOps := func(resolution string, minSec int, allowVideos bool) string {
		const ratios = `["16:9","9:16","1:1","4:3","3:4","21:9"]`
		dur := fmt.Sprintf(`{"name":"duration","type":"integer","minimum":%d,"maximum":30}`, minSec)
		res := fmt.Sprintf(`{"name":"resolution","type":"enum","enum":[%q],"fixed":%q}`, resolution, resolution)
		multiRef := `[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":30},{"slot":"reference_audios","max":10}]`
		if allowVideos {
			multiRef = `[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":30},{"slot":"reference_videos","max":10},{"slot":"reference_audios","max":10}]`
		}
		return fmt.Sprintf(`[
			{"id":"text_to_video","parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":%s}]},
			{"id":"first_frame","parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":["adaptive"],"fixed":"adaptive","default":"adaptive"}]},
			{"id":"first_last_frame","parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":["adaptive"],"fixed":"adaptive","default":"adaptive"}]},
			{"id":"video_multi_ref","inputs":%s,"parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":%s}]}
		]`, dur, res, ratios, dur, res, dur, res, multiRef, dur, res, ratios)
	}
	flashEcoOps := func(resolution string) string {
		const ratios = `["16:9","9:16","1:1","4:3","3:4","21:9"]`
		const dur = `{"name":"duration","type":"integer","minimum":4,"maximum":30}`
		res := fmt.Sprintf(`{"name":"resolution","type":"enum","enum":[%q],"fixed":%q}`, resolution, resolution)
		const multiRef = `[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":10},{"slot":"reference_audios","max":5}]`
		return fmt.Sprintf(`[
			{"id":"text_to_video","parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":%s}]},
			{"id":"first_frame","parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":["adaptive"],"fixed":"adaptive","default":"adaptive"}]},
			{"id":"first_last_frame","parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":["adaptive"],"fixed":"adaptive","default":"adaptive"}]},
			{"id":"video_multi_ref","inputs":%s,"parameters":[%s,%s,{"name":"aspect_ratio","type":"enum","enum":%s}]}
		]`, dur, res, ratios, dur, res, dur, res, multiRef, dur, res, ratios)
	}

	rulesDoc := fmt.Sprintf(`{"models":{
		"seedance-2-5":[
			{"channel_id":%[2]d,"channel_type":%[8]d,"profile":"seedance_openai_video","upstream_model":"BT-2.5-480p","base_url":%[1]q,"evidence":"fixture","operations":%[10]s},
			{"channel_id":%[3]d,"channel_type":%[8]d,"profile":"seedance_openai_video","upstream_model":"sd-2.5-720p-pro","base_url":%[1]q,"evidence":"fixture","operations":%[11]s},
			{"channel_id":%[4]d,"channel_type":%[8]d,"profile":"seedance_openai_video","upstream_model":"sd-2.5-1080p-plus","base_url":%[1]q,"evidence":"fixture","operations":%[12]s}
		],
		"seedance-2-5-flash":[
			{"channel_id":%[5]d,"channel_type":%[9]d,"profile":"apimart_video","request_model":"seedance-2-5-flash","upstream_model":"seedance-2-5-flash","base_url":%[1]q,"evidence":"fixture",
			 "operations":[
				{"id":"text_to_video","parameters":[{"name":"duration"},{"name":"resolution","fixed":"480p"},{"name":"aspect_ratio"}]},
				{"id":"first_frame","parameters":[{"name":"duration"},{"name":"resolution","fixed":"480p"}]},
				{"id":"first_last_frame","parameters":[{"name":"duration"},{"name":"resolution","fixed":"480p"}]},
				{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true},{"slot":"reference_videos"},{"slot":"reference_audios"}],"parameters":[{"name":"duration"},{"name":"resolution","fixed":"480p"},{"name":"aspect_ratio"}]},
				{"id":"video_edit","parameters":[{"name":"duration","fixed":-1},{"name":"resolution","fixed":"480p"}]},
				{"id":"video_extend","parameters":[{"name":"duration"},{"name":"resolution","fixed":"480p"}]}
			 ]},
			{"channel_id":%[6]d,"channel_type":%[8]d,"profile":"seedance_openai_video","upstream_model":"jiuyue-sd25-flash-480p","base_url":%[1]q,"evidence":"fixture","operations":%[13]s},
			{"channel_id":%[7]d,"channel_type":%[8]d,"profile":"seedance_openai_video","upstream_model":"jiuyue-sd25-flash-720p","base_url":%[1]q,"evidence":"fixture","operations":%[14]s}
		]
	}}`, h.upstreamURL, ch480Eco, ch720Std, ch1080Plus, chFlashNative, chFlash480Eco, chFlash720Eco,
		constant.ChannelTypeOpenAI, constant.ChannelTypeApiMart,
		seedanceOpenAIOps("480p", 5, false), seedanceOpenAIOps("720p", 5, false), seedanceOpenAIOps("1080p", 5, true),
		flashEcoOps("480p"), flashEcoOps("720p"))

	rules, err := modelcontracts.ReadVideoCapabilities(strings.NewReader(rulesDoc))
	require.NoError(t, err)
	modelcontracts.SetVideoCapabilities(rules)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	for _, tc := range []struct {
		modelID     string
		openAIGroup map[string]string
		allGroups   []string
	}{
		{
			modelID:     "seedance-2-5",
			openAIGroup: map[string]string{"seedance-2-5-480p-eco": "480p", "seedance-2-5-720p": "720p", "seedance-2-5-1080p-plus": "1080p"},
			allGroups:   []string{"seedance-2-5-480p-eco", "seedance-2-5-720p", "seedance-2-5-1080p-plus"},
		},
		{
			modelID:     "seedance-2-5-flash",
			openAIGroup: map[string]string{"seedance-2-5-flash-480p-eco": "480p", "seedance-2-5-flash-720p-eco": "720p"},
			allGroups:   []string{"seedance-2-5-flash-480p", "seedance-2-5-flash-480p-eco", "seedance-2-5-flash-720p-eco"},
		},
	} {
		pricingResp, contractResp := h.syntheticCatalogResponses(t, tc.modelID)
		data := contractResp["data"].(map[string]any)
		view := data["effective_video"].(map[string]any)
		assert.Equal(t, view, pricingResp["data"].([]map[string]any)[0]["effective_video"])
		published := view["groups"].(map[string]any)
		assert.ElementsMatch(t, tc.allGroups, slices.Collect(maps.Keys(published)))
		for groupName, wantRes := range tc.openAIGroup {
			variants := published[groupName].([]any)
			require.Len(t, variants, 4, groupName)
			for _, rawVariant := range variants {
				resParam := videoFixtureParameter(rawVariant.(map[string]any), "resolution")
				require.NotNil(t, resParam)
				assert.Equal(t, wantRes, resParam["default"], groupName)
				assert.NotContains(t, resParam, "required", groupName)
			}
		}
		encoded, err := common.Marshal(view)
		require.NoError(t, err)
		for _, private := range []string{"BT-2.5-480p", "sd-2.5-720p-pro", "sd-2.5-1080p-plus", "jiuyue-sd25-flash-", h.upstreamURL} {
			assert.NotContains(t, string(encoded), private)
		}
	}

	const refImage = "https://media.example/reference.png"
	const refVideo = "https://media.example/clip.mp4"
	const refAudio = "https://media.example/voice.mp3"

	t.Run("seedance-2-5 480p-eco omits resolution and forwards image+audio", func(t *testing.T) {
		before := h.attemptCount()
		resp := h.submit(t, "/v1/video/generations", "", map[string]any{
			"model":        "seedance-2-5@seedance-2-5-480p-eco",
			"operation":    "video_multi_ref",
			"prompt":       "lantern light",
			"duration":     6,
			"aspect_ratio": "16:9",
			"images":       []string{refImage},
			"audio_urls":   []string{refAudio},
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		attempts := h.attemptsFrom(before)
		require.Len(t, attempts, 1)
		assert.Equal(t, "s25-480p-eco", attempts[0].key)
		assert.Equal(t, "/v1/videos", attempts[0].path)
		assert.Equal(t, "BT-2.5-480p", attempts[0].body["model"])
		assert.Equal(t, "480p", attempts[0].body["resolution"])
		assert.EqualValues(t, 6, attempts[0].body["seconds"])
		assert.Equal(t, []any{refImage}, attempts[0].body["images"])
		assert.Equal(t, []any{refAudio}, attempts[0].body["audio_urls"])
		assert.NotContains(t, attempts[0].body, "operation")
	})

	t.Run("seedance-2-5 480p-eco rejects video_urls capability mismatch", func(t *testing.T) {
		h.requireMismatch(t, "/v1/video/generations", "", map[string]any{
			"model":        "seedance-2-5@seedance-2-5-480p-eco",
			"operation":    "video_multi_ref",
			"prompt":       "lantern light",
			"duration":     6,
			"aspect_ratio": "16:9",
			"images":       []string{refImage},
			"video_urls":   []string{refVideo},
		})
	})

	t.Run("seedance-2-5 1080p-plus forwards multimodal images, video_urls, and audio_urls", func(t *testing.T) {
		before := h.attemptCount()
		resp := h.submit(t, "/v1/videos", "", map[string]any{
			"model":        "seedance-2-5@seedance-2-5-1080p-plus",
			"operation":    "video_multi_ref",
			"prompt":       "rainstorm",
			"duration":     8,
			"aspect_ratio": "16:9",
			"images":       []string{refImage},
			"video_urls":   []string{refVideo},
			"audio_urls":   []string{refAudio},
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		attempts := h.attemptsFrom(before)
		require.Len(t, attempts, 1)
		assert.Equal(t, "s25-1080p-plus", attempts[0].key)
		assert.Equal(t, "/v1/videos", attempts[0].path)
		assert.Equal(t, "sd-2.5-1080p-plus", attempts[0].body["model"])
		assert.Equal(t, "1080p", attempts[0].body["resolution"])
		assert.EqualValues(t, 8, attempts[0].body["seconds"])
		assert.Equal(t, []any{refVideo}, attempts[0].body["video_urls"])
		assert.Equal(t, []any{refAudio}, attempts[0].body["audio_urls"])
	})

	t.Run("seedance-2-5-flash without explicit group selects tier group on /v1/video/generations", func(t *testing.T) {
		before := h.attemptCount()
		resp := h.submit(t, "/v1/video/generations", "", map[string]any{
			"model":        "seedance-2-5-flash",
			"operation":    "text_to_video",
			"prompt":       "mountain sunrise",
			"duration":     5,
			"resolution":   "480p",
			"aspect_ratio": "16:9",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		attempts := h.attemptsFrom(before)
		require.Len(t, attempts, 1)
		assert.Equal(t, "flash-480p-native", attempts[0].key)
		assert.Equal(t, "/v1/videos/generations", attempts[0].path)
		assert.Equal(t, "seedance-2-5-flash", attempts[0].body["model"])
		assert.Equal(t, "480p", attempts[0].body["resolution"])
		assert.Equal(t, "seedance-2-5-flash-480p", h.latestTask(t).Group)
	})

	t.Run("seedance-2-5-flash explicit eco group bypasses tier selector and injects resolution", func(t *testing.T) {
		before := h.attemptCount()
		resp := h.submit(t, "/v1/video/generations", "", map[string]any{
			"model":        "seedance-2-5-flash@seedance-2-5-flash-480p-eco",
			"operation":    "text_to_video",
			"prompt":       "desert dune",
			"duration":     5,
			"aspect_ratio": "16:9",
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		attempts := h.attemptsFrom(before)
		require.Len(t, attempts, 1)
		assert.Equal(t, "flash-480p-eco", attempts[0].key)
		assert.Equal(t, "/v1/videos", attempts[0].path)
		assert.Equal(t, "jiuyue-sd25-flash-480p", attempts[0].body["model"])
		assert.Equal(t, "480p", attempts[0].body["resolution"])
		assert.Equal(t, "seedance-2-5-flash-480p-eco", h.latestTask(t).Group)
	})

	t.Run("seedance-2-5-flash 720p-eco rejects video_edit, 1080p, and 480p mismatch with zero upstream calls", func(t *testing.T) {
		h.requireMismatch(t, "/v1/videos", "", map[string]any{
			"model":        "seedance-2-5-flash@seedance-2-5-flash-720p-eco",
			"operation":    "text_to_video",
			"prompt":       "mismatched resolution",
			"duration":     5,
			"resolution":   "480p",
			"aspect_ratio": "16:9",
		})
		for _, payload := range []map[string]any{
			{
				"model":      "seedance-2-5-flash@seedance-2-5-flash-720p-eco",
				"operation":  "video_edit",
				"prompt":     "edit clip",
				"duration":   5,
				"video_urls": []string{refVideo},
			},
			{
				"model":        "seedance-2-5-flash@seedance-2-5-flash-720p-eco",
				"operation":    "text_to_video",
				"prompt":       "too high res",
				"duration":     5,
				"resolution":   "1080p",
				"aspect_ratio": "16:9",
			},
		} {
			start := h.attemptCount()
			beforeBal, err := h.readBalances("/v1/videos", "")
			require.NoError(t, err)
			resp := h.submit(t, "/v1/videos", "", payload)
			require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
			assert.Equal(t, start, h.attemptCount())
			afterBal, err := h.readBalances("/v1/videos", "")
			require.NoError(t, err)
			assert.Equal(t, beforeBal, afterBal)
		}
	})
}

func TestSeedance25MixedProtocolGroupSelectsMultimodalDecoder(t *testing.T) {
	h := newVideoCapabilityRouteHarness(t)
	require.NoError(t, model.DB.Where("model = ?", "seedance-2-5").Delete(&model.Ability{}).Error)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"muse-video":0.01,"seedance-2-5":0.283}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"legacy-eco":0.2,"mixed-pro":0.55}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","legacy-eco":"Legacy Eco","mixed-pro":"Mixed Pro"}`))

	legacyEcoID := h.addSyntheticChannel(t, "legacy-eco-ch", "seedance-2-5", "legacy-eco", "fixture-eco-480p", constant.ChannelTypeOpenAI, 10)
	mixedNativeID := h.addSyntheticChannel(t, "mixed-native-ch", "seedance-2-5", "mixed-pro", "seedance-2-5", constant.ChannelTypeApiMart, 20)
	mixedOpenAIID := h.addSyntheticChannel(t, "mixed-openai-ch", "seedance-2-5", "mixed-pro", "sd-2.5-720p-pro", constant.ChannelTypeOpenAI, 10)

	rulesDoc := fmt.Sprintf(`{"models":{"seedance-2-5":[
		{"channel_id":%[2]d,"channel_type":%[5]d,"profile":"seedance25_openai_video","upstream_model":"fixture-eco-480p","base_url":%[1]q,"evidence":"fixture",
		 "operations":[{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true,"min":1,"max":1}],
		  "parameters":[{"name":"duration","fixed":30},{"name":"aspect_ratio","fixed":"9:16"},{"name":"resolution","fixed":"480p"}]}]},
		{"channel_id":%[3]d,"channel_type":%[6]d,"profile":"apimart_video","request_model":"seedance-2-5","upstream_model":"seedance-2-5","base_url":%[1]q,"evidence":"fixture",
		 "operations":[{"id":"text_to_video","parameters":[{"name":"duration"},{"name":"resolution","fixed":"720p"},{"name":"aspect_ratio"}]}]},
		{"channel_id":%[4]d,"channel_type":%[5]d,"profile":"seedance_openai_video","upstream_model":"sd-2.5-720p-pro","base_url":%[1]q,"evidence":"fixture",
		 "operations":[{"id":"text_to_video","parameters":[{"name":"duration","type":"integer","minimum":5,"maximum":30},{"name":"resolution","type":"enum","enum":["720p"],"fixed":"720p"},{"name":"aspect_ratio","type":"enum","enum":["16:9","9:16"]}]}]}
	]}}`, h.upstreamURL, legacyEcoID, mixedNativeID, mixedOpenAIID, constant.ChannelTypeOpenAI, constant.ChannelTypeApiMart)

	rules, err := modelcontracts.ReadVideoCapabilities(strings.NewReader(rulesDoc))
	require.NoError(t, err)
	modelcontracts.SetVideoCapabilities(rules)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	// Unheaded /v1/video/generations stays on native apimart_video for the mixed group.
	start := h.attemptCount()
	nativeResp := h.submit(t, "/v1/video/generations", "", map[string]any{
		"model":        "seedance-2-5@mixed-pro",
		"operation":    "text_to_video",
		"prompt":       "native route",
		"duration":     6,
		"resolution":   "720p",
		"aspect_ratio": "16:9",
	})
	require.Equal(t, http.StatusOK, nativeResp.Code, nativeResp.Body.String())
	attempts := h.attemptsFrom(start)
	require.Len(t, attempts, 1)
	assert.Equal(t, "mixed-native-ch", attempts[0].key)
	assert.Equal(t, "/v1/videos/generations", attempts[0].path)

	// /v1/videos and explicit openai_video header must select SeedanceMultimodalOpenAIDecoder
	// instead of falling back to legacy Seedance25OpenAIDecoder even though legacyEco is configured.
	for _, tc := range []struct {
		path, protocol string
	}{
		{"/v1/videos", ""},
		{"/v1/video/generations", modelcontracts.VideoProtocolOpenAI},
	} {
		before := h.attemptCount()
		resp := h.submitWithProtocol(t, tc.path, "", tc.protocol, map[string]any{
			"model":        "seedance-2-5@mixed-pro",
			"operation":    "text_to_video",
			"prompt":       "multimodal openai route",
			"duration":     6,
			"aspect_ratio": "16:9",
		})
		require.Equal(t, http.StatusOK, resp.Code, "%s: %s", tc.path, resp.Body.String())
		got := h.attemptsFrom(before)
		require.Len(t, got, 1)
		assert.Equal(t, "mixed-openai-ch", got[0].key)
		assert.Equal(t, "/v1/videos", got[0].path)
		assert.Equal(t, "sd-2.5-720p-pro", got[0].body["model"])
		assert.Equal(t, "720p", got[0].body["resolution"])
		assert.Equal(t, "16:9", got[0].body["aspect_ratio"])
		assert.EqualValues(t, 6, got[0].body["seconds"])
	}
}
