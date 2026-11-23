package apimart

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSubmitTaskID_Array(t *testing.T) {
	body := []byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_abc"}]}`)
	id, err := parseSubmitTaskID(body)
	require.NoError(t, err)
	assert.Equal(t, "task_abc", id)
}

func TestParseTaskResult_CompletedVideo(t *testing.T) {
	body := []byte(`{
	  "code":200,
	  "data":{
	    "id":"task_1",
	    "status":"completed",
	    "progress":100,
	    "result":{"videos":[{"url":"https://cdn.example/v.mp4"}]}
	  }
	}`)
	info, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://cdn.example/v.mp4", info.Url)
}

func TestParseTaskResult_CompletedImageURLArray(t *testing.T) {
	body := []byte(`{
	  "code":200,
	  "data":{
	    "id":"task_2",
	    "status":"completed",
	    "progress":100,
	    "result":{"images":[{"url":["https://cdn.example/i.png"]}]}
	  }
	}`)
	info, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://cdn.example/i.png", info.Url)
}

func TestParseTaskResult_CompletedMusic(t *testing.T) {
	body := []byte(`{
	  "code":200,
	  "data":{
	    "id":"task_music",
	    "status":"completed",
	    "progress":100,
	    "result":{"music":[{"audio_url":"https://cdn.example/song.mp3"}]}
	  }
	}`)
	info, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://cdn.example/song.mp3", info.Url)
}

func TestParseTaskResult_FailedFixture(t *testing.T) {
	body := []byte(`{"code":200,"data":{"id":"task_failed","status":"failed","error":{"message":"InvalidParameter.TaskTypeMismatch"}}}`)
	info, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	assert.Equal(t, "InvalidParameter.TaskTypeMismatch", info.Reason)
}

func TestParseTaskResult_QueuedFixture(t *testing.T) {
	body := []byte(`{"code":200,"data":{"id":"task_queued","status":"queued","progress":0}}`)
	info, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusQueued), info.Status)
}

func TestMapUpstreamModel(t *testing.T) {
	assert.Equal(t, "seedance-2.0", mapUpstreamModel("seedance-2-0"))
	assert.Equal(t, "seedance-2.0-fast", mapUpstreamModel("seedance-2-0-fast"))
	assert.Equal(t, "seedance-2.0-mini", mapUpstreamModel("seedance-2-0-mini"))
	assert.Equal(t, "seedance-2.5", mapUpstreamModel("seedance-2-5"))
	assert.Equal(t, "wan3.0-video", mapUpstreamModel("wan-3.0"))
	assert.Equal(t, "MiniMax-H3", mapUpstreamModel("minimax-h3"))
	assert.Equal(t, "grok-imagine-video-1.5", mapUpstreamModel("grok-imagine-video-1-5"))
	assert.Equal(t, "kling-v3", mapUpstreamModel("kling-v3"))
	assert.Equal(t, "midjourney", mapUpstreamModel("mj_imagine"))
	assert.Equal(t, "suno", mapUpstreamModel("suno-sounds"))
}

func TestDetectKind(t *testing.T) {
	assert.Equal(t, kindMusic, detectKind("suno", ""))
	assert.Equal(t, kindSoundEffect, detectKind("suno-sounds", ""))
	assert.Equal(t, kindSoundEffect, detectKind("suno", "/v1/music/generations/sounds"))
	assert.Equal(t, kindMidjourney, detectKind("midjourney", "/v1/images/generations"))
	assert.Equal(t, kindVideo, detectKind("kling-v3", "/v1/video/generations"))
}

func TestBuildRequestURL_SoundEffectSubPath(t *testing.T) {
	a := &TaskAdaptor{baseURL: "https://api.apimart.ai"}

	url, err := a.BuildRequestURL(&relaycommon.RelayInfo{OriginModelName: "suno-sounds"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.apimart.ai/v1/music/generations/sounds", url)

	url, err = a.BuildRequestURL(&relaycommon.RelayInfo{OriginModelName: "suno"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.apimart.ai/v1/music/generations", url)
}

func TestEstimateBilling_SoundEffectIsPerCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

	info := &relaycommon.RelayInfo{OriginModelName: "suno-sounds"}
	assert.Equal(t, map[string]float64{"units": 1}, (&TaskAdaptor{}).EstimateBilling(c, info))
}

func TestNormalizeEvoLinkWanRequestBody(t *testing.T) {
	body := map[string]any{
		"model":      "wan3.0-text-to-video",
		"prompt":     "a cat",
		"resolution": "480P",
		"seconds":    "4",
		"size":       "16:9",
	}
	normalizeEvoLinkWanRequestBody(body)
	assert.Equal(t, "480p", body["quality"])
	assert.NotContains(t, body, "resolution")
	assert.Equal(t, "4", body["duration"])
	assert.NotContains(t, body, "seconds")
	assert.Equal(t, "16:9", body["aspect_ratio"])
	assert.NotContains(t, body, "size")
}

func TestNormalizeEvoLinkWanRequestBodyKeepsExplicitUpstreamFields(t *testing.T) {
	body := map[string]any{
		"quality":      "1080P",
		"duration":     4,
		"seconds":      8,
		"aspect_ratio": "9:16",
		"size":         "16:9",
	}
	normalizeEvoLinkWanRequestBody(body)
	assert.Equal(t, "1080p", body["quality"])
	assert.Equal(t, 4, body["duration"])
	assert.NotContains(t, body, "seconds")
	assert.Equal(t, "9:16", body["aspect_ratio"])
	assert.NotContains(t, body, "size")
}

func TestIsEvoLinkWanUpstream(t *testing.T) {
	assert.True(t, isEvoLinkWanUpstream("wan3.0-text-to-video"))
	assert.True(t, isEvoLinkWanUpstream("wan3.0-prime-reference-video"))
	assert.False(t, isEvoLinkWanUpstream("wan30-720p"))
	assert.False(t, isEvoLinkWanUpstream("wan-3.0"))
}

func TestBuildRequestBody_SunoDurationByMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "prompt only",
			body: `{"model":"suno","prompt":"a cheerful summer pop song"}`,
			want: `{"model":"suno","prompt":"a cheerful summer pop song","version":"v6","custom":false}`,
		},
		{
			name: "implicit inspiration",
			body: `{"model":"suno","prompt":"instrumental workout music","duration":22}`,
			want: `{"model":"suno","prompt":"instrumental workout music","version":"v6","custom":false}`,
		},
		{
			name: "explicit inspiration",
			body: `{"model":"suno","prompt":"instrumental workout music","custom":false,"duration":22}`,
			want: `{"model":"suno","prompt":"instrumental workout music","version":"v6","custom":false}`,
		},
		{
			name: "custom lyrics",
			body: `{"model":"suno","prompt":"[Verse]\nA steady beat","custom":true,"duration":22,"version":"v6-mini"}`,
			want: `{"model":"suno","prompt":"[Verse]\nA steady beat","custom":true,"duration":22,"version":"v6-mini"}`,
		},
		{
			name: "sound effect",
			body: `{"model":"suno-sounds","prompt":"a distant bell","duration":12}`,
			want: `{"model":"suno","prompt":"a distant bell","duration":12}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{
				ChannelMeta:   &relaycommon.ChannelMeta{},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			}
			adaptor := &TaskAdaptor{}
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

			body, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			sent, err := io.ReadAll(body)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(sent))
		})
	}
}

func TestBuildRequestBody_SoundEffectKeepsPromptOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	c.Set("task_request_raw", map[string]any{"prompt": "A thunderstorm with distant bells"})

	info := &relaycommon.RelayInfo{
		OriginModelName: "suno-sounds",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&sent))
	assert.Equal(t, "suno", sent["model"])
	assert.Equal(t, "A thunderstorm with distant bells", sent["prompt"])
	assert.NotContains(t, sent, "version")
	assert.NotContains(t, sent, "custom")
}

// A channel that sets an explicit model_mapping must see that SKU on the wire,
// not the generic phase-one alias. EvoLink's real SKU for the public
// seedance-2-5 id is seedance-2.5-text-to-video; before this fix the phase-one
// table stamped the bare seedance-2.5 alias and upstream returned 404.
func TestBuildRequestBody_ChannelMappingOverridesPhaseOneWire(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil)
	c.Set("task_request_raw", map[string]any{
		"operation": "first_frame",
		"prompt":    "cat walks",
		"image_urls": []string{
			"https://cdn.example.com/a.png",
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-5",
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "seedance-2.5-text-to-video",
		},
	}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	var sent map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&sent))
	assert.Equal(t, "seedance-2.5-text-to-video", sent["model"])
}

// When the channel did not map the model, the phase-one wire table still
// supplies the upstream SKU.
func TestBuildRequestBody_PhaseOneWireFallbackWhenUnmapped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil)
	c.Set("task_request_raw", map[string]any{
		"operation": "first_frame",
		"prompt":    "cat walks",
		"image_urls": []string{
			"https://cdn.example.com/a.png",
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-5",
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped: false,
		},
	}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	var sent map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&sent))
	assert.Equal(t, "seedance-2.5", sent["model"])
}

func TestBuildRequestBody_StripsLocalOperationAndUsesOfficialModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil)
	c.Set("task_request_raw", map[string]any{
		"operation": "video_multi_ref",
		"prompt":    "reference",
		"image_urls": []string{
			"https://cdn.example.com/a.png",
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-1-5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-video",
		},
	}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	var sent map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&sent))
	assert.Equal(t, "grok-imagine-video-1.5", sent["model"])
	assert.NotContains(t, sent, "operation")
}

func TestTaskAdaptor_PolicyIsolationAndValidatedBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var adaptor channel.TaskAdaptor = &TaskAdaptor{}
	_, isValidatedBilling := adaptor.(channel.TaskValidatedBillingProvider)
	assert.True(t, isValidatedBilling)
	_, isValidatedFacts := adaptor.(channel.TaskValidatedUsageFactsProvider)
	assert.True(t, isValidatedFacts)

	newContext := func(payload string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		return c
	}

	t.Run("preset vs none isolates tier multiplier on identical seedance-2-5 480p media request", func(t *testing.T) {
		payload := `{"model":"seedance-2-5","operation":"first_frame","prompt":"animate","duration":5,"resolution":"480p","image_with_roles":[{"url":"https://cdn.example/a.png","role":"first_frame"}]}`

		presetCtx := newContext(payload)
		presetInfo := &relaycommon.RelayInfo{
			OriginModelName: "seedance-2-5",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName:    "seedance-2.5",
				ChannelOtherSettings: kitdto.ChannelOtherSettings{VideoTierBillingPolicy: kitdto.VideoTierBillingPolicyPreset},
			},
		}
		presetAdaptor := &TaskAdaptor{}
		require.Nil(t, presetAdaptor.ValidateRequestAndSetAction(presetCtx, presetInfo))
		presetRatios, err := presetAdaptor.EstimateBillingValidated(presetCtx, presetInfo)
		require.NoError(t, err)
		assert.Equal(t, 5.0, presetRatios["seconds"])
		assert.InDelta(t, 0.0576/0.216, presetRatios["tier"], 1e-9)
		assert.False(t, presetInfo.AutoDurationBilling)

		noneCtx := newContext(payload)
		noneInfo := &relaycommon.RelayInfo{
			OriginModelName: "seedance-2-5",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName:    "seedance-2.5",
				ChannelOtherSettings: kitdto.ChannelOtherSettings{VideoTierBillingPolicy: kitdto.VideoTierBillingPolicyNone},
			},
		}
		noneAdaptor := &TaskAdaptor{}
		require.Nil(t, noneAdaptor.ValidateRequestAndSetAction(noneCtx, noneInfo))
		noneRatios, err := noneAdaptor.EstimateBillingValidated(noneCtx, noneInfo)
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"seconds": 5}, noneRatios)
		assert.False(t, noneInfo.AutoDurationBilling)
	})

	t.Run("seedance-2-5 1080p passes under none and fails closed under preset", func(t *testing.T) {
		payload := `{"model":"seedance-2-5","operation":"text_to_video","prompt":"city","duration":5,"resolution":"1080p"}`

		noneCtx := newContext(payload)
		noneInfo := &relaycommon.RelayInfo{
			OriginModelName: "seedance-2-5",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName:    "seedance-2.5",
				ChannelOtherSettings: kitdto.ChannelOtherSettings{VideoTierBillingPolicy: kitdto.VideoTierBillingPolicyNone},
			},
		}
		noneAdaptor := &TaskAdaptor{}
		require.Nil(t, noneAdaptor.ValidateRequestAndSetAction(noneCtx, noneInfo))
		ratios, err := noneAdaptor.EstimateBillingValidated(noneCtx, noneInfo)
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"seconds": 5}, ratios)

		for _, policy := range []string{"", kitdto.VideoTierBillingPolicyPreset} {
			presetCtx := newContext(payload)
			presetInfo := &relaycommon.RelayInfo{
				OriginModelName: "seedance-2-5",
				TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName:    "seedance-2.5",
					ChannelOtherSettings: kitdto.ChannelOtherSettings{VideoTierBillingPolicy: policy},
				},
			}
			presetAdaptor := &TaskAdaptor{}
			taskErr := presetAdaptor.ValidateRequestAndSetAction(presetCtx, presetInfo)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			assert.Contains(t, taskErr.Message, `video_tier_billing_policy="none"`)

			_, err := presetAdaptor.EstimateBillingValidated(presetCtx, presetInfo)
			require.Error(t, err)
			assert.Nil(t, presetAdaptor.EstimateBilling(presetCtx, presetInfo), "invalid input must never fall back to default 5s")
		}
	})

	t.Run("none policy still enforces video contract and duration upper bound", func(t *testing.T) {
		for _, payload := range []string{
			`{"model":"seedance-2-5","operation":"text_to_video","prompt":"city","duration":5,"resolution":"999p"}`,
			`{"model":"seedance-2-5","operation":"text_to_video","prompt":"city","duration":31,"resolution":"1080p"}`,
			`{"model":"kling-v3","prompt":"city","duration":3601}`,
		} {
			c := newContext(payload)
			info := &relaycommon.RelayInfo{
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelOtherSettings: kitdto.ChannelOtherSettings{VideoTierBillingPolicy: kitdto.VideoTierBillingPolicyNone},
				},
			}
			a := &TaskAdaptor{}
			taskErr := a.ValidateRequestAndSetAction(c, info)
			require.NotNil(t, taskErr, payload)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		}
	})

	t.Run("invalid runtime policy fails closed in both validation and estimate", func(t *testing.T) {
		payload := `{"model":"seedance-2-5","operation":"text_to_video","prompt":"city","duration":5,"resolution":"720p"}`
		c := newContext(payload)
		info := &relaycommon.RelayInfo{
			OriginModelName: "seedance-2-5",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName:    "seedance-2.5",
				ChannelOtherSettings: kitdto.ChannelOtherSettings{VideoTierBillingPolicy: "invalid_policy"},
			},
		}
		a := &TaskAdaptor{}
		taskErr := a.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)

		_, err := a.EstimateBillingValidated(c, info)
		require.Error(t, err)
		assert.Nil(t, a.EstimateBilling(c, info))
	})

	t.Run("validation and estimation share rateModel for wan-3.0 and fixed SKU", func(t *testing.T) {
		fixedCtx := newContext(`{"model":"wan-3.0","operation":"text_to_video","prompt":"city","duration":5,"resolution":"720P"}`)
		fixedInfo := &relaycommon.RelayInfo{
			OriginModelName: "wan-3.0",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "wan30-720p"},
		}
		fixedAdaptor := &TaskAdaptor{}
		require.Nil(t, fixedAdaptor.ValidateRequestAndSetAction(fixedCtx, fixedInfo))
		ratios, err := fixedAdaptor.EstimateBillingValidated(fixedCtx, fixedInfo)
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"seconds": 5}, ratios)

		unmappedCtx := newContext(`{"model":"wan-3.0","operation":"text_to_video","prompt":"city","duration":5,"resolution":"480P"}`)
		unmappedInfo := &relaycommon.RelayInfo{
			OriginModelName: "wan-3.0",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta:     &relaycommon.ChannelMeta{},
		}
		unmappedAdaptor := &TaskAdaptor{}
		require.Nil(t, unmappedAdaptor.ValidateRequestAndSetAction(unmappedCtx, unmappedInfo))
		ratios, err = unmappedAdaptor.EstimateBillingValidated(unmappedCtx, unmappedInfo)
		require.NoError(t, err)
		assert.Equal(t, 5.0, ratios["seconds"])
		assert.InDelta(t, 0.25, ratios["tier"], 1e-9)
	})

	t.Run("auto duration flag sets true only for duration -1 and resets on explicit duration", func(t *testing.T) {
		autoCtx := newContext(`{"model":"seedance-2-5","operation":"text_to_video","prompt":"city","duration":-1,"resolution":"720p"}`)
		info := &relaycommon.RelayInfo{
			OriginModelName: "seedance-2-5",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.5"},
		}
		a := &TaskAdaptor{}
		require.Nil(t, a.ValidateRequestAndSetAction(autoCtx, info))
		assert.True(t, info.AutoDurationBilling)

		ratios, err := a.EstimateBillingValidated(autoCtx, info)
		require.NoError(t, err)
		assert.Equal(t, 30.0, ratios["seconds"])
		assert.True(t, info.AutoDurationBilling)

		facts, err := a.ExtractUsageFactsValidated(autoCtx, info)
		require.NoError(t, err)
		assert.Equal(t, float64(30), facts["seconds"])
		assert.True(t, info.AutoDurationBilling)

		explicitCtx := newContext(`{"model":"seedance-2-5","operation":"text_to_video","prompt":"city","duration":6,"resolution":"720p"}`)
		require.Nil(t, a.ValidateRequestAndSetAction(explicitCtx, info))
		assert.False(t, info.AutoDurationBilling)
		ratios, err = a.EstimateBillingValidated(explicitCtx, info)
		require.NoError(t, err)
		assert.Equal(t, 6.0, ratios["seconds"])
		assert.False(t, info.AutoDurationBilling)
	})
}

func TestParseTaskResult_ActualVideoDuration(t *testing.T) {
	a := &TaskAdaptor{}

	validCases := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "videos[0].duration number",
			body: `{"code":200,"data":{"id":"t1","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4","duration":8}]}}}`,
			want: 8,
		},
		{
			name: "videos[0].seconds string",
			body: `{"code":200,"data":{"id":"t2","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4","seconds":"8.5"}]}}}`,
			want: 8.5,
		},
		{
			name: "result.video.duration",
			body: `{"code":200,"data":{"id":"t3","status":"completed","result":{"video":{"url":"https://cdn.example/v.mp4","duration":12}}}}`,
			want: 12,
		},
		{
			name: "result.actual_duration",
			body: `{"code":200,"data":{"id":"t4","status":"completed","result":{"video_url":"https://cdn.example/v.mp4","actual_duration":10}}}`,
			want: 10,
		},
		{
			name: "data.usage.seconds",
			body: `{"code":200,"data":{"id":"t5","status":"completed","usage":{"seconds":9},"result":{"videos":[{"url":"https://cdn.example/v.mp4"}]}}}`,
			want: 9,
		},
		{
			name: "data.duration",
			body: `{"code":200,"data":{"id":"t6","status":"completed","duration":7,"result":{"videos":[{"url":"https://cdn.example/v.mp4"}]}}}`,
			want: 7,
		},
		{
			name: "flat top-level duration",
			body: `{"id":"t7","status":"completed","duration":6,"result":{"videos":[{"url":"https://cdn.example/v.mp4"}]}}`,
			want: 6,
		},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := a.ParseTaskResult(nil, nil, []byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
			require.NotNil(t, info.UsageFacts)
			assert.Equal(t, tc.want, info.UsageFacts["seconds"])
		})
	}

	omittedOrInvalidCases := []struct {
		name string
		body string
	}{
		{
			name: "missing duration",
			body: `{"code":200,"data":{"id":"t1","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4"}]}}}`,
		},
		{
			name: "zero duration",
			body: `{"code":200,"data":{"id":"t2","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4","duration":0}]}}}`,
		},
		{
			name: "negative duration",
			body: `{"code":200,"data":{"id":"t3","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4","duration":-5}]}}}`,
		},
		{
			name: "corrupt video duration does not fall through to data.duration",
			body: `{"code":200,"data":{"id":"t4","status":"completed","duration":10,"result":{"videos":[{"url":"https://cdn.example/v.mp4","duration":-1}]}}}`,
		},
		{
			name: "NaN string duration",
			body: `{"code":200,"data":{"id":"t5","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4","duration":"NaN"}]}}}`,
		},
		{
			name: "oversized duration above MaxTaskDurationSeconds",
			body: `{"code":200,"data":{"id":"t6","status":"completed","result":{"videos":[{"url":"https://cdn.example/v.mp4","duration":3601}]}}}`,
		},
		{
			name: "failed status ignores duration",
			body: `{"code":200,"data":{"id":"t7","status":"failed","duration":8,"error":{"message":"boom"}}}`,
		},
		{
			name: "in-progress status ignores duration",
			body: `{"code":200,"data":{"id":"t8","status":"processing","duration":30}}`,
		},
		{
			name: "music task does not populate video seconds",
			body: `{"code":200,"data":{"id":"t9","status":"completed","duration":120,"result":{"music":[{"audio_url":"https://cdn.example/a.mp3","duration":120}]}}}`,
		},
		{
			name: "image task does not populate video seconds",
			body: `{"code":200,"data":{"id":"t10","status":"completed","duration":5,"result":{"images":[{"url":"https://cdn.example/i.png"}]}}}`,
		},
	}

	for _, tc := range omittedOrInvalidCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := a.ParseTaskResult(nil, nil, []byte(tc.body))
			require.NoError(t, err)
			assert.Nil(t, info.UsageFacts)
		})
	}
}

func TestAdjustBillingOnComplete_AutoDuration(t *testing.T) {
	a := &TaskAdaptor{}

	t.Run("recomputes quota for actual seconds under ModelPrice without tier", func(t *testing.T) {
		// ModelPrice=0.01 ($0.01/s = 5000 quota/s), GroupRatio=0.75 => 3750 quota/s.
		// Pre-consumed 30s = 112500; actual 8s = 30000.
		task := &model.Task{
			TaskID: "task_auto_none",
			Status: model.TaskStatusSuccess,
			Quota:  112500,
			PrivateData: model.TaskPrivateData{
				BillingContext: &model.TaskBillingContext{
					ModelPrice:          0.01,
					GroupRatio:          0.75,
					OtherRatios:         map[string]float64{"seconds": 30},
					PerCallBilling:      true,
					AutoDurationBilling: true,
				},
			},
		}
		got := a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusSuccess),
			UsageFacts: map[string]any{"seconds": float64(8)},
		})
		assert.Equal(t, 30000, got)
	})

	t.Run("preserves tier multiplier during auto-duration reconciliation", func(t *testing.T) {
		// ModelPrice=0.216, GroupRatio=0.5 => base=54000; tier=0.09608/0.216.
		// Pre-consumed 30s = 720600; actual 8s = 192160.
		tier := 0.09608 / 0.216
		task := &model.Task{
			TaskID: "task_auto_tier",
			Status: model.TaskStatusSuccess,
			Quota:  720600,
			PrivateData: model.TaskPrivateData{
				BillingContext: &model.TaskBillingContext{
					ModelPrice:          0.216,
					GroupRatio:          0.5,
					OtherRatios:         map[string]float64{"seconds": 30, "tier": tier},
					PerCallBilling:      true,
					AutoDurationBilling: true,
				},
			},
		}
		got := a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusSuccess),
			UsageFacts: map[string]any{"seconds": float64(8)},
		})
		assert.Equal(t, 192160, got)
	})

	t.Run("recomputes quota under ModelRatio mode", func(t *testing.T) {
		// ModelRatio=0.02 => base = 0.02/2 * 500000 * 1 = 5000 quota/s.
		// Pre-consumed 30s = 150000; actual 10s = 50000.
		task := &model.Task{
			TaskID: "task_auto_ratio",
			Status: model.TaskStatusSuccess,
			Quota:  150000,
			PrivateData: model.TaskPrivateData{
				BillingContext: &model.TaskBillingContext{
					ModelPrice:          -1,
					ModelRatio:          0.02,
					GroupRatio:          1.0,
					OtherRatios:         map[string]float64{"seconds": 30},
					AutoDurationBilling: true,
				},
			},
		}
		got := a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusSuccess),
			UsageFacts: map[string]any{"seconds": float64(10)},
		})
		assert.Equal(t, 50000, got)
	})

	t.Run("returns 0 for explicit-duration tasks, failures, free group, invalid/missing/exceeding duration, and overflow", func(t *testing.T) {
		baseTask := func() *model.Task {
			return &model.Task{
				TaskID: "task_guard",
				Status: model.TaskStatusSuccess,
				Quota:  150000,
				PrivateData: model.TaskPrivateData{
					BillingContext: &model.TaskBillingContext{
						ModelPrice:          0.01,
						GroupRatio:          1.0,
						OtherRatios:         map[string]float64{"seconds": 30},
						PerCallBilling:      true,
						AutoDurationBilling: true,
					},
				},
			}
		}

		// AutoDurationBilling == false
		explicit := baseTask()
		explicit.PrivateData.BillingContext.AutoDurationBilling = false
		assert.Equal(t, 0, a.AdjustBillingOnComplete(explicit, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusSuccess),
			UsageFacts: map[string]any{"seconds": float64(8)},
		}))

		// Failure status
		failed := baseTask()
		failed.Status = model.TaskStatusFailure
		assert.Equal(t, 0, a.AdjustBillingOnComplete(failed, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusFailure),
			UsageFacts: map[string]any{"seconds": float64(8)},
		}))

		// Free group (GroupRatio <= 0)
		freeGroup := baseTask()
		freeGroup.Quota = 0
		freeGroup.PrivateData.BillingContext.GroupRatio = 0
		assert.Equal(t, 0, a.AdjustBillingOnComplete(freeGroup, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusSuccess),
			UsageFacts: map[string]any{"seconds": float64(8)},
		}))

		// Missing, zero, negative, NaN, Inf, equal to 30, exceeding 30, exceeding MaxTaskDurationSeconds
		for _, facts := range []map[string]any{
			nil,
			{},
			{"seconds": float64(0)},
			{"seconds": float64(-5)},
			{"seconds": math.NaN()},
			{"seconds": math.Inf(1)},
			{"seconds": float64(30)},
			{"seconds": float64(45)},
			{"seconds": float64(relaycommon.MaxTaskDurationSeconds + 1)},
		} {
			assert.Equal(t, 0, a.AdjustBillingOnComplete(baseTask(), &relaycommon.TaskInfo{
				Status:     string(model.TaskStatusSuccess),
				UsageFacts: facts,
			}), "facts=%v", facts)
		}

		// Overflow protection (QuotaFromFloatChecked clamp returns 0, never negative)
		overflow := baseTask()
		overflow.PrivateData.BillingContext.ModelPrice = float64(common.MaxQuota)
		assert.Equal(t, 0, a.AdjustBillingOnComplete(overflow, &relaycommon.TaskInfo{
			Status:     string(model.TaskStatusSuccess),
			UsageFacts: map[string]any{"seconds": float64(8)},
		}))
	})
}
