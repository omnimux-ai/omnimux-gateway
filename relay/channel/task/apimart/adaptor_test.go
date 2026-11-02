package apimart

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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

func TestBuildRequestBody_MusicInjectsLiveSunoVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	c.Set("task_request_raw", map[string]any{"prompt": "a cheerful summer pop song"})

	info := &relaycommon.RelayInfo{
		OriginModelName: "suno",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&sent))
	// Upstream retired v4; the injected default must be a version it still serves.
	assert.Equal(t, "v6", sent["version"])
	assert.Equal(t, false, sent["custom"])
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
