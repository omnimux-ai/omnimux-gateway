package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGrokImagineUpstreamModel(t *testing.T) {
	assert.Equal(t, "grok-imagine-video-1.5", grokImagineUpstreamModel("grok-imagine-video-1-5"))
	assert.Equal(t, "grok-imagine-video", grokImagineUpstreamModel("grok-imagine-video"))
	assert.Equal(t, "grok-imagine-video-1.5", grokImagineUpstreamModel("grok-imagine-video-1.5"))
}

func TestIsGrokImagineVideo(t *testing.T) {
	assert.True(t, isGrokImagineVideo(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-video-1-5"}))
	assert.True(t, isGrokImagineVideo(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-video"}))
	assert.False(t, isGrokImagineVideo(&relaycommon.RelayInfo{OriginModelName: "sora-2"}))
	assert.False(t, isGrokImagineVideo(nil))
}

func TestGrokSubmitTaskID(t *testing.T) {
	assert.Equal(t, "abc", grokSubmitTaskID(grokSubmitResponse{RequestID: "abc"}))
	assert.Equal(t, "id-1", grokSubmitTaskID(grokSubmitResponse{ID: "id-1"}))
	assert.Equal(t, "", grokSubmitTaskID(grokSubmitResponse{}))
}

func TestParseGrokPoll_SuccessUrlEmpty(t *testing.T) {
	info := parseGrokPoll(grokPollResponse{
		Status: "done",
		Video:  &grokPollVideo{URL: "http://127.0.0.1:8000/v1/media/videos/vid_123.mp4", Duration: 5},
	})
	assert.Equal(t, model.TaskStatusSuccess, info.Status)
	assert.Equal(t, "", info.Url)
	assert.Equal(t, "100%", info.Progress)
}

func TestParseGrokPoll_Progress100(t *testing.T) {
	info := parseGrokPoll(grokPollResponse{
		Status: "completed",
		Video:  &grokPollVideo{URL: "http://grok2api:8000/v1/media/videos/vid_456.mp4", Duration: 5},
	})
	assert.Equal(t, model.TaskStatusSuccess, info.Status)
	assert.Equal(t, "", info.Url)
	assert.Equal(t, "100%", info.Progress)
}

func TestConvertToOpenAIVideo_ProxyURL(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		TaskID: "task_demo",
		Data:   []byte(`{"id":"req_upstream","status":"done","video":{"url":"http://127.0.0.1:8000/v1/media/videos/vid_123.mp4","duration":5}}`),
	}
	out, err := a.ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	expectedProxyURL := taskcommon.BuildProxyURL("task_demo")
	assert.Equal(t, "task_demo", gjson.GetBytes(out, "id").String())
	assert.Equal(t, expectedProxyURL, gjson.GetBytes(out, "video.url").String())
	assert.NotContains(t, string(out), "127.0.0.1")
	assert.NotContains(t, string(out), "grok2api")
}

func TestConvertToOpenAIVideo_TopLevelURL(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		TaskID: "task_demo_top",
		Data:   []byte(`{"id":"req_upstream","status":"done","url":"http://127.0.0.1:8000/v1/media/videos/vid_123.mp4"}`),
	}
	out, err := a.ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	expectedProxyURL := taskcommon.BuildProxyURL("task_demo_top")
	assert.Equal(t, "task_demo_top", gjson.GetBytes(out, "id").String())
	assert.Equal(t, expectedProxyURL, gjson.GetBytes(out, "url").String())
	assert.NotContains(t, string(out), "127.0.0.1")
}

func TestParseGrokPollExpired(t *testing.T) {
	info := parseGrokPoll(grokPollResponse{Status: "expired"})
	assert.Equal(t, model.TaskStatusFailure, info.Status)
	assert.Equal(t, "request expired", info.Reason)
}

func TestParseTaskResultGrokPending(t *testing.T) {
	body, err := common.Marshal(map[string]any{"status": "pending", "progress": 12})
	require.NoError(t, err)
	info, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusQueued, info.Status)
	assert.Equal(t, "12%", info.Progress)
}

func TestBuildRequestURLGrokImagine(t *testing.T) {
	a := &TaskAdaptor{baseURL: "http://grok2api:8000"}
	url, err := a.BuildRequestURL(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-video-1-5"})
	require.NoError(t, err)
	assert.Equal(t, "http://grok2api:8000/v1/videos/generations", url)
}

func TestBuildRequestURLSoraUnchanged(t *testing.T) {
	a := &TaskAdaptor{baseURL: "https://api.openai.com"}
	url, err := a.BuildRequestURL(&relaycommon.RelayInfo{OriginModelName: "sora-2"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/videos", url)
}

func TestEstimateBillingGrokImagineIsPerCall(t *testing.T) {
	a := &TaskAdaptor{}
	ratios := a.EstimateBilling(nil, &relaycommon.RelayInfo{OriginModelName: "grok-imagine-video-1-5"})
	assert.Equal(t, map[string]float64{"units": 1}, ratios)
}
