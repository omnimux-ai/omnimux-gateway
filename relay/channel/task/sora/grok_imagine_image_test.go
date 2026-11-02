package sora

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsGrokImagineImage(t *testing.T) {
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-image"}))
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-image-2-0"}))
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-image-lite"}))
	assert.True(t, IsAsyncImageModel("grok-imagine-image"))
	assert.True(t, IsAsyncImageModel("grok-imagine-image-2-0"))
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "gemini-3.1-flash-image"}))
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "gemini-3.1-flash-lite-image"}))
	assert.True(t, IsAsyncImageModel("gemini-3.1-flash-image"))
	assert.True(t, IsAsyncImageModel("gemini-3.1-flash-lite-image"))
	assert.False(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "dall-e-3"}))
	assert.False(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "gpt-image-1"}))
	assert.False(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "sora-2"}))
	assert.False(t, IsGrokImagineImage(nil))
	assert.False(t, IsAsyncImageModel("dall-e-3"))
}

func TestGrokImagineImageUpstreamModel(t *testing.T) {
	assert.Equal(t, "grok-imagine-image-2.0", grokImagineImageUpstreamModel("grok-imagine-image-2-0"))
	assert.Equal(t, "grok-imagine-image-lite", grokImagineImageUpstreamModel("grok-imagine-image-lite"))
}

func TestLooksLikeSyncImageResponse(t *testing.T) {
	// 1. grok2api direct sync response with URL
	syncBody := []byte(`{
		"created": 1783860000,
		"data": [
			{"url": "http://127.0.0.1:8000/v1/media/images/img_123.jpg"}
		]
	}`)
	url, isSync := looksLikeSyncImageResponse(syncBody)
	assert.True(t, isSync)
	assert.Equal(t, "http://127.0.0.1:8000/v1/media/images/img_123.jpg", url)

	// 2. sync response with b64_json
	b64Body := []byte(`{
		"created": 1783860000,
		"data": [
			{"b64_json": "iVBORw0KGgoAAAANS..."}
		]
	}`)
	_, isSyncB64 := looksLikeSyncImageResponse(b64Body)
	assert.True(t, isSyncB64)

	// 3. async response is not sync
	asyncBody := []byte(`{
		"code": 200,
		"data": [
			{"status": "submitted", "task_id": "task_abc123"}
		]
	}`)
	_, isSyncAsync := looksLikeSyncImageResponse(asyncBody)
	assert.False(t, isSyncAsync)
}

func TestRewriteSyncImageResponse(t *testing.T) {
	prevServer := system_setting.ServerAddress
	system_setting.ServerAddress = "https://omnimux.ai"
	t.Cleanup(func() { system_setting.ServerAddress = prevServer })

	syncBody := []byte(`{
		"created": 1783860000,
		"data": [
			{"url": "http://127.0.0.1:8000/v1/media/images/img_123.jpg", "revised_prompt": "a sunny beach"}
		]
	}`)
	rewritten, origURL, err := rewriteSyncImageResponse(syncBody, "task_pub_999")
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8000/v1/media/images/img_123.jpg", origURL)

	var parsed struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL           string `json:"url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	err = common.Unmarshal(rewritten, &parsed)
	require.NoError(t, err)
	assert.Equal(t, int64(1783860000), parsed.Created)
	require.Len(t, parsed.Data, 1)
	assert.Equal(t, "https://omnimux.ai/v1/images/task_pub_999/content", parsed.Data[0].URL)
	assert.Equal(t, "a sunny beach", parsed.Data[0].RevisedPrompt)
}

func TestLooksLikeAsyncImageSubmit(t *testing.T) {
	// 1. Apimart / Evolink format
	body1 := []byte(`{
		"code": 200,
		"data": [
			{"status": "submitted", "task_id": "task_evolink_123"}
		]
	}`)
	taskID, ok := looksLikeAsyncImageSubmit(body1)
	assert.True(t, ok)
	assert.Equal(t, "task_evolink_123", taskID)

	// 2. Flat format
	body2 := []byte(`{
		"id": "task_flat_456",
		"status": "queued"
	}`)
	taskID2, ok2 := looksLikeAsyncImageSubmit(body2)
	assert.True(t, ok2)
	assert.Equal(t, "task_flat_456", taskID2)

	// 3. Sync response should not match async submit
	body3 := []byte(`{
		"created": 1783860000,
		"data": [
			{"url": "http://127.0.0.1:8000/v1/media/images/img_123.jpg"}
		]
	}`)
	_, ok3 := looksLikeAsyncImageSubmit(body3)
	assert.False(t, ok3)
}

func TestParseImageTaskPollApimart(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"data": {
			"task_id": "task_img_789",
			"status": "completed",
			"progress": 100,
			"result": {
				"images": ["https://cdn.apimart.ai/img_final.png"]
			}
		}
	}`)
	info, err := parseImageTaskPoll(body)
	require.NoError(t, err)
	assert.Equal(t, "task_img_789", info.TaskID)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://cdn.apimart.ai/img_final.png", info.Url)
	assert.Equal(t, "100%", info.Progress)
}

// TestParseImageTaskPollApimartArrayURL covers the real APIMart shape where
// result.images contains objects whose "url" field is itself an array:
//   {"result":{"images":[{"url":["https://..."]}]}}
func TestParseImageTaskPollApimartArrayURL(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"data": {
			"id": "task_01M1JZA2EZR79QDV9ATPZV5JKS",
			"status": "completed",
			"progress": 100,
			"result": {
				"images": [{"url": ["https://getapib.org/image/9998211582970689-x.jpg"], "expires_at": 1788503429}]
			}
		}
	}`)
	info, err := parseImageTaskPoll(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://getapib.org/image/9998211582970689-x.jpg", info.Url)
	assert.Equal(t, "100%", info.Progress)
}

func TestParseImageTaskPollEvolinkProgress(t *testing.T) {
	body := []byte(`{
		"task_id": "task_evo_456",
		"status": "in_progress",
		"progress": 40
	}`)
	info, err := parseImageTaskPoll(body)
	require.NoError(t, err)
	assert.Equal(t, "task_evo_456", info.TaskID)
	assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
	assert.Equal(t, "40%", info.Progress)
}

func TestParseImageTaskPollFailure(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"data": {
			"task_id": "task_fail_1",
			"status": "failed",
			"error": {
				"message": "content filtered"
			}
		}
	}`)
	info, err := parseImageTaskPoll(body)
	require.NoError(t, err)
	assert.Equal(t, "task_fail_1", info.TaskID)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	assert.Equal(t, "content filtered", info.Reason)
}

func TestTaskAdaptorImageMethods(t *testing.T) {
	a := &TaskAdaptor{baseURL: "http://grok2api:8000"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_pub_image_1",
		},
	}

	// 1. BuildRequestURL
	reqURL, err := a.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "http://grok2api:8000/v1/images/generations", reqURL)

	// 2. EstimateBilling
	billing := a.EstimateBilling(nil, info)
	assert.Equal(t, map[string]float64{"units": 1}, billing)

	// 3. DoResponse with sync response (grok2api direct)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	syncRespBody := `{"created":123,"data":[{"url":"http://127.0.0.1:8000/v1/media/images/test.jpg"}]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(syncRespBody)),
		Header:     make(http.Header),
	}
	upstreamID, taskData, taskErr := a.DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "task_pub_image_1", upstreamID)
	assert.Contains(t, string(taskData), "content")
	assert.Equal(t, 0, w.Body.Len(), "DoResponse must not write directly to client writer")

	// Verify ParseResponse wraps ClientResponse and sets Immediate success
	respParse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(syncRespBody)),
		Header:     make(http.Header),
	}
	parsed, parseErr := a.ParseResponse(c, respParse, info)
	require.Nil(t, parseErr)
	require.NotNil(t, parsed)
	assert.NotNil(t, parsed.ClientResponse)
	assert.Contains(t, string(parsed.ClientResponse.([]byte)), "content")
	require.NotNil(t, parsed.Immediate)
	assert.Equal(t, string(model.TaskStatusSuccess), parsed.Immediate.Status)

	// 4. DoResponse with async submit (Evolink / Apimart)
	wAsync := httptest.NewRecorder()
	cAsync, _ := gin.CreateTestContext(wAsync)

	asyncRespBody := `{"code":200,"data":[{"status":"submitted","task_id":"task_evo_submit_1"}]}`
	respAsync := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(asyncRespBody)),
		Header:     make(http.Header),
	}
	upstreamIDAsync, _, taskErrAsync := a.DoResponse(cAsync, respAsync, info)
	require.Nil(t, taskErrAsync)
	assert.Equal(t, "task_evo_submit_1", upstreamIDAsync)
	assert.Equal(t, 0, wAsync.Body.Len(), "DoResponse async submit must not write directly to client writer")
}

// TestParseImageTaskPollEvolinkResults covers the real Evolink shape:
//   {"id":"task-...","type":"image","status":"completed","results":["https://..."],"result_data":[{"url":"..."}]}
func TestParseImageTaskPollEvolinkResults(t *testing.T) {
	body := []byte(`{
		"id": "task-abc123",
		"type": "image",
		"model": "grok-imagine-image-2.0",
		"object": "image.generation.task",
		"status": "completed",
		"created": 1788418736,
		"results": ["https://files.evolink.ai/004WHG72X3R4OM77IN/images/2026/09/03/6e5a1b40.jpg"],
		"duration": 90,
		"progress": 100,
		"task_info": {"can_cancel": false},
		"result_data": [{"url": "https://files.evolink.ai/004WHG72X3R4OM77IN/images/2026/09/03/6e5a1b40.jpg", "mime_type": "image/jpeg"}]
	}`)
	info, err := parseImageTaskPoll(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://files.evolink.ai/004WHG72X3R4OM77IN/images/2026/09/03/6e5a1b40.jpg", info.Url)
	assert.Equal(t, "100%", info.Progress)
}

func TestSyncImageResponseIncludesTaskIDAndID(t *testing.T) {
	syncRespBody := []byte(`{"created":1790658579,"data":[{"url":"https://example.com/test.png","b64_json":"abcd"}]}`)
	rewritten, origURL, err := rewriteSyncImageResponse(syncRespBody, "task_sync_test_999")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/test.png", origURL)

	var envelope map[string]any
	require.NoError(t, common.Unmarshal(rewritten, &envelope))
	assert.Equal(t, "task_sync_test_999", envelope["task_id"])
	assert.Equal(t, "task_sync_test_999", envelope["id"])
}

