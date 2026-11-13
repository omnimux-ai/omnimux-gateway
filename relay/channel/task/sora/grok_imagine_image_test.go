package sora

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "nano-banana-2-lite"}))
	assert.True(t, IsAsyncImageModel("gemini-3.1-flash-image"))
	assert.True(t, IsAsyncImageModel("nano-banana-2-lite"))
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
//
//	{"result":{"images":[{"url":["https://..."]}]}}
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
//
//	{"id":"task-...","type":"image","status":"completed","results":["https://..."],"result_data":[{"url":"..."}]}
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

func TestSeedreamImageModelRecognition(t *testing.T) {
	assert.True(t, IsAsyncImageModel("seedream-5-5"))
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "seedream-5-5"}))
	assert.True(t, IsSeedreamImageModel(&relaycommon.RelayInfo{OriginModelName: "seedream-5-5"}))
	assert.True(t, IsSeedreamImageModel(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedream-5-5"}}))
	assert.False(t, IsSeedreamImageModel(&relaycommon.RelayInfo{OriginModelName: "seedream-5-0-pro"}))
	assert.False(t, IsSeedreamImageModel(nil))
}

func TestSeedreamSizeRatio(t *testing.T) {
	cases := map[string]float64{
		"1K":   1,
		"1.5K": 1,
		"2K":   1.75,
		"2k":   1.75,
		"":     1.75, // omitted size bills the documented 2K default
	}
	for input, want := range cases {
		assert.Equal(t, want, seedreamSizeRatio(input), input)
	}
}

func TestSeedreamEstimateBilling(t *testing.T) {
	cases := []struct {
		size string
		want float64
	}{
		{"1K", 1},
		{"1.5K", 1},
		{"2K", 1.75},
		{"", 1.75},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("task_request", relaycommon.TaskSubmitReq{Size: tc.size})
		info := &relaycommon.RelayInfo{OriginModelName: "seedream-5-5"}
		ratios := (&TaskAdaptor{}).EstimateBilling(c, info)
		assert.Equal(t, tc.want, ratios["size"], tc.size)
	}
}

func TestValidateSeedreamImageRequest(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantReject bool
	}{
		{"text-to-image 2K ok", `{"model":"seedream-5-5","prompt":"a cat","size":"2K","n":1}`, false},
		{"omitted size ok", `{"prompt":"a cat"}`, false},
		{"reference image_urls rejected", `{"prompt":"a cat","size":"2K","image_urls":["https://x/y.png"]}`, true},
		{"reference image string rejected", `{"prompt":"a cat","size":"2K","image":"https://x/y.png"}`, true},
		{"layer decomposition rejected", `{"prompt":"a cat","size":"2K","layer_decomposition":true}`, true},
		{"nested metadata image_urls rejected", `{"prompt":"a cat","metadata":{"image_urls":["https://x/y.png"]}}`, true},
		{"nested parameters n rejected", `{"prompt":"a cat","parameters":{"n":2}}`, true},
		{"n not 1 rejected", `{"prompt":"a cat","size":"2K","n":2}`, true},
		{"illegal size rejected", `{"prompt":"a cat","size":"4K"}`, true},
		{"pixel size rejected", `{"prompt":"a cat","size":"2048x2048"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{OriginModelName: "seedream-5-5"}
			taskErr := validateSeedreamImageRequest(c, info, relaycommon.TaskSubmitReq{})
			if tc.wantReject {
				require.NotNil(t, taskErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			} else {
				require.Nil(t, taskErr)
			}
		})
	}
}

// TestSeedreamPriceTierAnchors pins the official USD anchors the ModelPrice and
// size multiplier must reproduce: base 1K/1.5K = $0.02, 2K = $0.035.
func TestSeedreamPriceTierAnchors(t *testing.T) {
	const lowTierUSD = 0.02
	assert.InDelta(t, 0.02, lowTierUSD*seedreamSizeRatio("1K"), 1e-9)
	assert.InDelta(t, 0.02, lowTierUSD*seedreamSizeRatio("1.5K"), 1e-9)
	assert.InDelta(t, 0.035, lowTierUSD*seedreamSizeRatio("2K"), 1e-9)
}

func TestSeedreamCanonicalSize(t *testing.T) {
	cases := map[string]string{
		"1K": "1K", "1k": "1K",
		"1.5K": "1.5K", "1.5k": "1.5K",
		"2K": "2K", "2k": "2K",
		"": "2K",
	}
	for input, want := range cases {
		assert.Equal(t, want, seedreamCanonicalSize(input), input)
	}
}

// TestReferenceURLsFromBodyObjects guards the Finding 1 hardening: object-shaped
// references must be recognised so the MVP gate can reject them instead of
// forwarding them upstream under text-to-image pricing.
func TestReferenceURLsFromBodyObjects(t *testing.T) {
	cases := []map[string]any{
		{"image_urls": []any{map[string]any{"url": "https://x/a.png"}}},
		{"images": []any{map[string]any{"image_url": "https://x/b.png"}}},
		{"input_reference": map[string]any{"url": "https://x/c.png"}},
		{"image": map[string]any{"image_url": "https://x/d.png"}},
	}
	for _, body := range cases {
		assert.NotEmpty(t, referenceURLsFromBody(body), "%v", body)
	}
}

func TestValidateSeedreamImageRequestObjectReferences(t *testing.T) {
	bodies := []string{
		`{"prompt":"a cat","size":"2K","image_urls":[{"url":"https://x/a.png"}]}`,
		`{"prompt":"a cat","size":"2K","images":[{"image_url":"https://x/b.png"}]}`,
		`{"prompt":"a cat","size":"2K","input_reference":{"url":"https://x/c.png"}}`,
		`{"prompt":"a cat","size":"2K","images":["https://x/d.png"]}`,
		`{"prompt":"a cat","size":"2K","input_reference":"https://x/e.png"}`,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{OriginModelName: "seedream-5-5"}
			taskErr := validateSeedreamImageRequest(c, info, relaycommon.TaskSubmitReq{})
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		})
	}
}

func TestValidateSeedreamImageRequestSetsGenerateAction(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(`{"prompt":"a cat","size":"2K"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{OriginModelName: "seedream-5-5", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	require.Nil(t, validateSeedreamImageRequest(c, info, relaycommon.TaskSubmitReq{Prompt: "a cat", Size: "2K"}))
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
}

// TestSeedreamForwardedSizeMatchesBillingTier guards the F3 residual: the size
// forwarded upstream must be the same tier billing charges, including when the
// caller supplies only a resolution.
func TestSeedreamForwardedSizeMatchesBillingTier(t *testing.T) {
	cases := []struct {
		name string
		req  relaycommon.TaskSubmitReq
		want string
	}{
		{"size 2K", relaycommon.TaskSubmitReq{Size: "2K"}, "2K"},
		{"size 1k", relaycommon.TaskSubmitReq{Size: "1k"}, "1K"},
		{"resolution only 1K", relaycommon.TaskSubmitReq{Resolution: "1K"}, "1K"},
		{"resolution only 1.5k", relaycommon.TaskSubmitReq{Resolution: "1.5k"}, "1.5K"},
		{"resolution only 2K", relaycommon.TaskSubmitReq{Resolution: "2K"}, "2K"},
		{"empty", relaycommon.TaskSubmitReq{}, "2K"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("task_request", tc.req)
			assert.Equal(t, tc.want, seedreamCanonicalSize(seedreamRequestedSize(c)))
		})
	}
}

func TestNanoBanana21ImageModelRecognition(t *testing.T) {
	assert.True(t, IsAsyncImageModel("nano-banana-2-1"))
	assert.True(t, IsGrokImagineImage(&relaycommon.RelayInfo{OriginModelName: "nano-banana-2-1"}))
	assert.True(t, IsNanoBanana21ImageModel(&relaycommon.RelayInfo{OriginModelName: "nano-banana-2-1"}))
	assert.False(t, IsNanoBanana21ImageModel(&relaycommon.RelayInfo{OriginModelName: "nano-banana-2"}))
	assert.False(t, IsNanoBanana21ImageModel(nil))
}

func TestNanoBanana21ResolutionRatio(t *testing.T) {
	cases := map[string]float64{
		"1K": 1,
		"2K": 0.051 / 0.034,
		"4K": 0.114 / 0.034,
		"2k": 0.051 / 0.034,
		"":   0.051 / 0.034, // omitted resolution bills the explicit 2K default
	}
	for input, want := range cases {
		assert.InDelta(t, want, nanoBanana21ResolutionRatio(input), 1e-12, input)
	}
}

func TestNanoBanana21EstimateBilling(t *testing.T) {
	cases := []struct {
		resolution string
		want       float64
	}{
		{"1K", 1},
		{"2K", 0.051 / 0.034},
		{"4K", 0.114 / 0.034},
		{"", 0.051 / 0.034},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("task_request", relaycommon.TaskSubmitReq{Resolution: tc.resolution})
		info := &relaycommon.RelayInfo{OriginModelName: "nano-banana-2-1"}
		ratios := (&TaskAdaptor{}).EstimateBilling(c, info)
		assert.InDelta(t, tc.want, ratios["size"], 1e-12, tc.resolution)
	}
}

// TestNanoBanana21PriceTierAnchors pins the official Vertex USD anchors the
// 1K base ModelPrice and resolution multiplier must reproduce.
func TestNanoBanana21PriceTierAnchors(t *testing.T) {
	const base1KUSD = 0.034
	assert.InDelta(t, 0.034, base1KUSD*nanoBanana21ResolutionRatio("1K"), 1e-9)
	assert.InDelta(t, 0.051, base1KUSD*nanoBanana21ResolutionRatio("2K"), 1e-9)
	assert.InDelta(t, 0.114, base1KUSD*nanoBanana21ResolutionRatio("4K"), 1e-9)
}

func TestNanoBanana21CanonicalResolution(t *testing.T) {
	cases := map[string]string{
		"1K": "1K", "1k": "1K",
		"2K": "2K", "2k": "2K",
		"4K": "4K", "4k": "4K",
		"": "2K",
	}
	for input, want := range cases {
		assert.Equal(t, want, nanoBanana21CanonicalResolution(input), input)
	}
}

func TestValidateNanoBanana21ImageRequest(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantReject bool
	}{
		{"text-to-image 2K ok", `{"model":"nano-banana-2-1","prompt":"a cat","resolution":"2K","n":1}`, false},
		{"omitted resolution ok", `{"prompt":"a cat"}`, false},
		{"auto size ok", `{"prompt":"a cat","size":"auto","resolution":"1K"}`, false},
		{"aspect ratio ok", `{"prompt":"a cat","size":"16:9","resolution":"4K"}`, false},
		{"0.5K rejected", `{"prompt":"a cat","resolution":"0.5K"}`, true},
		{"3K rejected", `{"prompt":"a cat","resolution":"3K"}`, true},
		{"illegal size rejected", `{"prompt":"a cat","size":"7:3"}`, true},
		{"pixel size rejected", `{"prompt":"a cat","size":"1024x1024"}`, true},
		{"n not 1 rejected", `{"prompt":"a cat","resolution":"2K","n":2}`, true},
		{"image_urls rejected", `{"prompt":"a cat","resolution":"2K","image_urls":["https://x/y.png"]}`, true},
		{"image string rejected", `{"prompt":"a cat","resolution":"2K","image":"https://x/y.png"}`, true},
		{"layer decomposition rejected", `{"prompt":"a cat","resolution":"2K","layer_decomposition":true}`, true},
		{"nested metadata image_urls rejected", `{"prompt":"a cat","metadata":{"image_urls":["https://x/y.png"]}}`, true},
		{"nested parameters n rejected", `{"prompt":"a cat","parameters":{"n":2}}`, true},
		{"nested metadata layer decomposition rejected", `{"prompt":"a cat","metadata":{"layer_decomposition":true}}`, true},
		{"deep nested image_urls rejected", `{"prompt":"a cat","metadata":{"parameters":{"image_urls":["https://x/y.png"]}}}`, true},
		{"model_params rejected", `{"prompt":"a cat","model_params":{"web_search":true}}`, true},
		{"nested parameters n=1 ok", `{"prompt":"a cat","parameters":{"n":1}}`, false},
		{"official_fallback true rejected", `{"prompt":"a cat","resolution":"2K","official_fallback":true}`, true},
		{"official_fallback false ok", `{"prompt":"a cat","resolution":"2K","official_fallback":false}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{OriginModelName: "nano-banana-2-1"}
			taskErr := validateNanoBanana21ImageRequest(c, info, relaycommon.TaskSubmitReq{})
			if tc.wantReject {
				require.NotNil(t, taskErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			} else {
				require.Nil(t, taskErr)
			}
		})
	}
}

func TestValidateNanoBanana21ImageRequestSetsGenerateAction(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewBufferString(`{"prompt":"a cat","resolution":"2K"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{OriginModelName: "nano-banana-2-1", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	require.Nil(t, validateNanoBanana21ImageRequest(c, info, relaycommon.TaskSubmitReq{Prompt: "a cat", Resolution: "2K"}))
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
}

// TestNanoBanana21BuildRequestBodyMapsResolutionAndDisablesFallback guards R5 and
// R3: APIMart Ext must receive `resolution` plus official_fallback=false (never a
// token-priced fallback), while EvoLink receives the same tier as `quality`.
func TestNanoBanana21BuildRequestBodyMapsResolutionAndDisablesFallback(t *testing.T) {
	t.Run("apimart ext uses resolution and disables fallback", func(t *testing.T) {
		info := newImageRelayInfo("nano-banana-2-1", "")
		info.UpstreamModelName = "gemini-nano-banana-2.1-ext"
		ctx := newJSONImageContext(t, "/v1/images/generations", `{"model":"nano-banana-2-1","prompt":"a cat","size":"auto"}`)
		reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
		require.NoError(t, err)
		encoded, err := io.ReadAll(reader)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, common.Unmarshal(encoded, &got))
		assert.Equal(t, "gemini-nano-banana-2.1-ext", got["model"])
		assert.Equal(t, "2K", got["resolution"])
		assert.Equal(t, false, got["official_fallback"])
		_, hasQuality := got["quality"]
		assert.False(t, hasQuality)
	})

	t.Run("evolink uses quality for the same tier", func(t *testing.T) {
		info := newImageRelayInfo("nano-banana-2-1", "")
		info.UpstreamModelName = "gemini-nano-banana-2.1"
		ctx := newJSONImageContext(t, "/v1/images/generations", `{"model":"nano-banana-2-1","prompt":"a cat","size":"auto","official_fallback":false}`)
		ctx.Set("task_request", relaycommon.TaskSubmitReq{Resolution: "4K"})
		reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
		require.NoError(t, err)
		encoded, err := io.ReadAll(reader)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, common.Unmarshal(encoded, &got))
		assert.Equal(t, "gemini-nano-banana-2.1", got["model"])
		assert.Equal(t, "4K", got["quality"])
		_, hasResolution := got["resolution"]
		assert.False(t, hasResolution)
		_, hasFallback := got["official_fallback"]
		assert.False(t, hasFallback)
	})
}

// TestValidateImageMVPRejectsMultipart guards F1: a multipart request hides its
// file parts from the JSON validator, so a reference image uploaded as image /
// input_reference / file must be rejected as a whole for both MVP image models.
func TestValidateImageMVPRejectsMultipart(t *testing.T) {
	for _, model := range []string{"nano-banana-2-1", "seedream-5-5"} {
		for _, field := range []string{"image", "input_reference", "file"} {
			t.Run(model+"/"+field, func(t *testing.T) {
				var buf bytes.Buffer
				mw := multipart.NewWriter(&buf)
				require.NoError(t, mw.WriteField("model", model))
				require.NoError(t, mw.WriteField("prompt", "a cat"))
				fw, err := mw.CreateFormFile(field, "ref.png")
				require.NoError(t, err)
				_, err = fw.Write([]byte("png"))
				require.NoError(t, err)
				require.NoError(t, mw.Close())

				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", &buf)
				c.Request.Header.Set("Content-Type", mw.FormDataContentType())
				info := &relaycommon.RelayInfo{OriginModelName: model}
				info.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: model}
				taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
				require.NotNil(t, taskErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			})
		}
	}
}
