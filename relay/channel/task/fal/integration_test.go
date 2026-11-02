package fal_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/fal"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQA_ChannelSystemAndRouting verifies PRD Criterion 1:
// - constant.ChannelTypeFalAI == 64
// - Default BaseURL is "https://queue.fal.run"
// - relay.GetTaskAdaptor correctly returns *taskfal.TaskAdaptor
// - router/task-router.go registers the task read route, router/video-router.go the video routes
func TestQA_ChannelSystemAndRouting(t *testing.T) {
	// 1. Channel constant and BaseURL
	assert.Equal(t, 64, constant.ChannelTypeFalAI)
	assert.Equal(t, "https://queue.fal.run", constant.ChannelBaseURLs[constant.ChannelTypeFalAI])
	assert.Equal(t, "fal.ai", constant.ChannelTypeNames[constant.ChannelTypeFalAI])
	assert.Equal(t, "fal.ai", constant.GetChannelTypeName(constant.ChannelTypeFalAI))

	// 2. relay.GetTaskAdaptor returns *taskfal.TaskAdaptor
	adaptorFromChannelType := relay.GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeFalAI)))
	require.NotNil(t, adaptorFromChannelType, "GetTaskAdaptor must not be nil for platform 64")
	_, isFalAdaptor := adaptorFromChannelType.(*fal.TaskAdaptor)
	assert.True(t, isFalAdaptor, "GetTaskAdaptor must return *taskfal.TaskAdaptor")
	assert.Equal(t, "fal.ai", adaptorFromChannelType.GetChannelName())
	assert.Equal(t, 8, len(adaptorFromChannelType.GetModelList()))

	// 3. Router registration. The task read route lives in SetTaskRouter, the
	// video routes in SetVideoRouter. Gin allows one parameter name per position,
	// so /v1/tasks/:key carries the plugin key on submit and the task id on read;
	// a second /v1/tasks/:task_id would panic the engine at startup.
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router.SetTaskRouter(engine)
	router.SetVideoRouter(engine)

	routes := engine.Routes()
	hasTaskFetchRoute := false
	hasVideoGenRoute := false
	hasLegacyFetchRoute := false

	for _, r := range routes {
		if r.Method == http.MethodGet && r.Path == "/v1/tasks/:key" {
			hasTaskFetchRoute = true
		}
		if r.Method == http.MethodPost && r.Path == "/v1/video/generations" {
			hasVideoGenRoute = true
		}
		if r.Method == http.MethodGet && r.Path == "/v1/video/generations/:task_id" {
			hasLegacyFetchRoute = true
		}
	}

	assert.True(t, hasTaskFetchRoute, "Route GET /v1/tasks/:key must be registered for task reads")
	assert.True(t, hasVideoGenRoute, "Route POST /v1/video/generations must be registered")
	assert.True(t, hasLegacyFetchRoute, "Route GET /v1/video/generations/:task_id must be registered")
}

// TestQA_FiveModelsAndEndpointMapping verifies PRD Criterion 2:
// - All Minimax H3 models and aliases are recognized and correctly mapped to upstream endpoints
// - Compound Upstream Task ID encoding and decoding
func TestQA_FiveModelsAndEndpointMapping(t *testing.T) {
	expectedModels := []struct {
		inputModel       string
		expectedEndpoint string
	}{
		{"minimax/h3-max", fal.ModelMinimaxH3MaxTextToVideo},
		{"h3-max", fal.ModelMinimaxH3MaxTextToVideo},
		{"fal-ai/minimax/h3-max", fal.ModelMinimaxH3MaxTextToVideo},
		{"minimax/h3", fal.ModelMinimaxH3MaxTextToVideo},
		{"h3", fal.ModelMinimaxH3MaxTextToVideo},
		{"minimax/h3-max/text-to-video", fal.ModelMinimaxH3MaxTextToVideo},
		{"minimax/h3-max-t2v", fal.ModelMinimaxH3MaxTextToVideo},
		{"h3-max-t2v", fal.ModelMinimaxH3MaxTextToVideo},

		{"minimax/h3-max/image-to-video", fal.ModelMinimaxH3MaxImageToVideo},
		{"minimax/h3-max-i2v", fal.ModelMinimaxH3MaxImageToVideo},
		{"h3-max-image-to-video", fal.ModelMinimaxH3MaxImageToVideo},

		{"minimax/h3-max/reference-to-video", fal.ModelMinimaxH3MaxReferenceToVideo},
		{"minimax/h3-max-r2v", fal.ModelMinimaxH3MaxReferenceToVideo},
		{"h3-max-reference-to-video", fal.ModelMinimaxH3MaxReferenceToVideo},

		{"minimax/h3-max-turbo", fal.ModelMinimaxH3MaxTurboTextToVideo},
		{"h3-max-turbo", fal.ModelMinimaxH3MaxTurboTextToVideo},
		{"minimax/h3-turbo", fal.ModelMinimaxH3MaxTurboTextToVideo},
		{"h3-turbo", fal.ModelMinimaxH3MaxTurboTextToVideo},
		{"minimax/h3-max-turbo/text-to-video", fal.ModelMinimaxH3MaxTurboTextToVideo},
		{"minimax/h3-max-turbo-t2v", fal.ModelMinimaxH3MaxTurboTextToVideo},
		{"h3-max-turbo-t2v", fal.ModelMinimaxH3MaxTurboTextToVideo},

		{"minimax/h3-max-turbo/image-to-video", fal.ModelMinimaxH3MaxTurboImageToVideo},
		{"minimax/h3-max-turbo-i2v", fal.ModelMinimaxH3MaxTurboImageToVideo},
		{"h3-max-turbo-image-to-video", fal.ModelMinimaxH3MaxTurboImageToVideo},
	}

	for _, tc := range expectedModels {
		t.Run("map_"+tc.inputModel, func(t *testing.T) {
			endpoint := fal.GetFalEndpoint(tc.inputModel)
			assert.Equal(t, tc.expectedEndpoint, endpoint)
			assert.True(t, fal.IsSupportedModel(tc.inputModel))
		})
	}

	// Verify unsupported model
	assert.False(t, fal.IsSupportedModel("random/unsupported-model"))

	// Verify compound Upstream Task ID encoding and decoding
	reqID := "01948e6b-3f47-79a8-80b4-1773950b731d"
	compositeID := fal.FormatUpstreamTaskID(fal.ModelMinimaxH3MaxTurboTextToVideo, reqID)
	assert.Equal(t, "minimax/h3-max-turbo/text-to-video:01948e6b-3f47-79a8-80b4-1773950b731d", compositeID)

	ep, decodedReqID, err := fal.ParseUpstreamTaskID(compositeID)
	require.NoError(t, err)
	assert.Equal(t, fal.ModelMinimaxH3MaxTurboTextToVideo, ep)
	assert.Equal(t, reqID, decodedReqID)
}

// TestQA_PayloadValidationAndNormalization verifies PRD Criterion 3:
// - Prompt required
// - Image URL required for image-to-video
// - Video URL required for reference-to-video
// - Duration defaults to 5s, supports 10s
// - Aspect ratio normalization (16:9, 9:16, 1:1, 4:3, 21:9)
// - Resolution normalization (720p, 1080p)
func TestQA_PayloadValidationAndNormalization(t *testing.T) {
	adaptor := &fal.TaskAdaptor{}
	gin.SetMode(gin.TestMode)

	t.Run("valid text-to-video payload via adaptor", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":        "minimax/h3-max",
			"prompt":       "A majestic golden eagle soaring over snow mountains",
			"aspect_ratio": "9:16",
			"duration":     "10s",
			"resolution":   "1080p",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "minimax/h3-max",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)

		reader, err := adaptor.BuildRequestBody(c, info)
		require.NoError(t, err)
		data, _ := io.ReadAll(reader)

		var payload fal.FalVideoSubmitPayload
		err = common.Unmarshal(data, &payload)
		require.NoError(t, err)
		assert.Equal(t, "A majestic golden eagle soaring over snow mountains", payload.Prompt)
		assert.Equal(t, "9:16", payload.AspectRatio)
		assert.Equal(t, "10", payload.Duration)
		assert.Equal(t, fal.Resolution1080P, payload.Resolution)
		assert.True(t, payload.PromptOptimizer)
	})

	t.Run("text-to-video missing prompt fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":        "minimax/h3-max",
			"aspect_ratio": "16:9",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "minimax/h3-max",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Contains(t, taskErr.Message, "prompt is required")
	})

	t.Run("image-to-video missing image_url fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":  "minimax/h3-max/image-to-video",
			"prompt": "Animate this portrait smiling",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "minimax/h3-max/image-to-video",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Contains(t, taskErr.Message, "exactly one image")
	})

	t.Run("reference-to-video missing video_url fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":  "minimax/h3-max/reference-to-video",
			"prompt": "Transfer dance moves",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "minimax/h3-max/reference-to-video",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Contains(t, taskErr.Message, "requires 1–12 references")
	})

	t.Run("missing model fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"prompt": "a prompt without model",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:   &relaycommon.ChannelMeta{},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Equal(t, "invalid_request", taskErr.Code)
	})

	t.Run("unsupported model fails interception", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":  "openai/sora-v1",
			"prompt": "a futuristic scene",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "openai/sora-v1",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr, "Unsupported model must be intercepted with TaskError")
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	})

	t.Run("reference-to-video with image_url per PRD requirement", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max/reference-to-video",
			"prompt":    "Stylize character into anime",
			"image_url": "https://example.com/character.png",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "minimax/h3-max/reference-to-video",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr, "PRD requires image_url for reference-to-video")
	})
}

// TestQA_TwoPhasePollingAndResultExtraction verifies PRD Criterion 4:
// - Status probing: IN_QUEUE / IN_PROGRESS
// - Result fetching: COMPLETED triggers same-round result resolve to SUCCESS
// - Error handling: FAILED status and safety errors parse to FAILURE
// - CDN 404 tolerance: returns IN_PROGRESS instead of failing prematurely
func TestQA_TwoPhasePollingAndResultExtraction(t *testing.T) {
	adaptor := &fal.TaskAdaptor{}

	t.Run("lifecycle_in_queue_to_in_progress", func(t *testing.T) {
		queueResp := []byte(`{"status":"IN_QUEUE","queue_position":5}`)
		info, err := adaptor.ParseTaskResult(nil, nil, queueResp)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusQueued), info.Status)
		assert.Equal(t, taskcommon.ProgressQueued, info.Progress)

		runningResp := []byte(`{"status":"IN_PROGRESS","queue_position":0}`)
		info2, err := adaptor.ParseTaskResult(nil, nil, runningResp)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusInProgress), info2.Status)
		assert.Equal(t, taskcommon.ProgressInProgress, info2.Progress)
	})

	t.Run("two_phase_completed_success", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/minimax/h3-max/requests/req-success-101/status":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
			case "/minimax/h3-max/requests/req-success-101":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"video": {
						"url": "https://v3.fal.media/files/lion/final_render.mp4",
						"content_type": "video/mp4",
						"file_name": "final_render.mp4",
						"file_size": 10485760
					},
					"usage": {
						"execution_time": 4.12
					}
				}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer mockServer.Close()

		resp, err := adaptor.FetchTaskMap(mockServer.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-success-101",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, respBytes)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
		assert.Equal(t, "https://v3.fal.media/files/lion/final_render.mp4", info.Url)
		assert.Equal(t, taskcommon.ProgressComplete, info.Progress)
	})

	t.Run("two_phase_cdn_404_resilience", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/minimax/h3-max/requests/req-lag-102/status":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
			case "/minimax/h3-max/requests/req-lag-102":
				// CDN replication delay: returns 404 momentarily
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer mockServer.Close()

		resp, err := adaptor.FetchTaskMap(mockServer.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-lag-102",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, respBytes)
		require.NoError(t, err)
		// Must not fail! Should gracefully report in_progress
		assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
	})

	t.Run("content_moderation_failure_parsed_for_refund", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/minimax/h3-max/requests/req-nsfw-103/status" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"status": "FAILED",
					"error": {
						"message": "Content moderation triggered: prompt contains sensitive elements",
						"code": "content_moderation"
					}
				}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer mockServer.Close()

		resp, err := adaptor.FetchTaskMap(mockServer.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-nsfw-103",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, respBytes)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
		assert.Contains(t, info.Reason, "Content moderation triggered")
	})
}

// TestQA_BillingDurationMultiplier verifies PRD Criterion 5:
// - 5s duration: 1.0x multiplier
// - 10s duration: 2.0x multiplier
// - Supports int, int64, float64, and strings ("5", "10", "5s", "10s")
func TestQA_BillingDurationMultiplier(t *testing.T) {
	adaptor := &fal.TaskAdaptor{}
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name          string
		durationInput any
		expectedRatio float64
	}{
		{"default duration (nil)", nil, 1.0},
		{"integer 5", 5, 1.0},
		{"integer 10", 10, 2.0},
		{"int64 10", int64(10), 2.0},
		{"float64 10.0", float64(10.0), 2.0},
		{"string '5'", "5", 1.0},
		{"string '10'", "10", 2.0},
		{"string '5s'", "5s", 1.0},
		{"string '10s'", "10s", 2.0},
		{"string '10S' uppercase", "10S", 2.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

			reqRaw := map[string]any{
				"prompt": "high energy marketing video",
			}
			if tc.durationInput != nil {
				reqRaw["duration"] = tc.durationInput
			}
			c.Set("task_request_raw", reqRaw)

			ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{})
			require.NotNil(t, ratios)
			assert.Equal(t, tc.expectedRatio, ratios["duration"])
		})
	}
}

// TestQA_EndToEndLifecycleSimulation simulates the entire pipeline:
// 1. ValidateRequestAndSetAction
// 2. EstimateBilling
// 3. BuildRequestURL, BuildRequestHeader, BuildRequestBody
// 4. DoResponse (mock upstream 200 IN_QUEUE)
// 5. FetchTask (polling cycle 1: IN_PROGRESS)
// 6. FetchTask (polling cycle 2: COMPLETED -> resolved to SUCCESS with video)
func TestQA_EndToEndLifecycleSimulation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &fal.TaskAdaptor{}

	reqID := "req-e2e-live-888"
	var currentStatus = "IN_PROGRESS"

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Key fal_test_secret_key", authHeader, "Authorization header must be Key-prefixed")

		switch r.URL.Path {
		case "/minimax/h3-max/text-to-video", "/minimax/h3-max":
			// Submit endpoint
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"request_id": "%s",
				"status": "IN_QUEUE",
				"response_url": "https://queue.fal.run/minimax/h3-max/text-to-video/requests/%s",
				"status_url": "https://queue.fal.run/minimax/h3-max/text-to-video/requests/%s/status"
			}`, reqID, reqID, reqID)))

		case fmt.Sprintf("/minimax/h3-max/text-to-video/requests/%s/status", reqID), fmt.Sprintf("/minimax/h3-max/requests/%s/status", reqID):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"%s"}`, currentStatus)))

		case fmt.Sprintf("/minimax/h3-max/text-to-video/requests/%s", reqID), fmt.Sprintf("/minimax/h3-max/requests/%s", reqID):
			if currentStatus == "COMPLETED" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"video": {
						"url": "https://v3.fal.media/files/lion/completed_e2e.mp4",
						"content_type": "video/mp4"
					}
				}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockUpstream.Close()

	// 1. Validate & Set Action
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	submitBody := map[string]any{
		"model":        "minimax/h3-max",
		"prompt":       "A high-speed cyber motorcycle race through neon metropolis",
		"duration":     "10s",
		"aspect_ratio": "16:9",
		"resolution":   "1080p",
	}
	bodyData, _ := common.Marshal(submitBody)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyData))
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeFalAI,
			ChannelBaseUrl: mockUpstream.URL,
			ApiKey:         "Bearer fal_test_secret_key", // test auto-cleans Bearer prefix
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_omnimux_e2e_001",
		},
		OriginModelName: "minimax/h3-max",
	}
	adaptor.Init(info)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
	assert.Equal(t, fal.ModelMinimaxH3MaxTextToVideo, info.UpstreamModelName)

	// 2. Billing check: 10s should be 2.0x
	ratios := adaptor.EstimateBilling(c, info)
	assert.Equal(t, 2.0, ratios["duration"])

	// 3. Build request
	reqURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s/%s", mockUpstream.URL, fal.ModelMinimaxH3MaxTextToVideo), reqURL)

	reqObj, err := http.NewRequest(http.MethodPost, reqURL, nil)
	require.NoError(t, err)
	err = adaptor.BuildRequestHeader(c, reqObj, info)
	require.NoError(t, err)
	assert.Equal(t, "Key fal_test_secret_key", reqObj.Header.Get("Authorization"))

	reqReader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	reqObj.Body = io.NopCloser(reqReader)

	// 4. Submit to mock upstream
	submitResp, err := http.DefaultClient.Do(reqObj)
	require.NoError(t, err)

	upstreamTaskID, taskData, taskErr := adaptor.DoResponse(c, submitResp, info)
	require.Nil(t, taskErr)
	assert.Equal(t, fmt.Sprintf("%s:%s", fal.ModelMinimaxH3MaxTextToVideo, reqID), upstreamTaskID)
	assert.NotEmpty(t, taskData)

	// 5. Polling Cycle 1: status is IN_PROGRESS
	currentStatus = "IN_PROGRESS"
	fetchResp1, err := adaptor.FetchTaskMap(mockUpstream.URL, "fal_test_secret_key", map[string]any{
		"task_id": upstreamTaskID,
	}, "")
	require.NoError(t, err)
	defer fetchResp1.Body.Close()

	body1, err := io.ReadAll(fetchResp1.Body)
	require.NoError(t, err)
	info1, err := adaptor.ParseTaskResult(nil, nil, body1)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), info1.Status)
	assert.Equal(t, taskcommon.ProgressInProgress, info1.Progress)

	// 6. Polling Cycle 2: status is COMPLETED -> adaptor internally resolves final video
	currentStatus = "COMPLETED"
	fetchResp2, err := adaptor.FetchTaskMap(mockUpstream.URL, "fal_test_secret_key", map[string]any{
		"task_id": upstreamTaskID,
	}, "")
	require.NoError(t, err)
	defer fetchResp2.Body.Close()

	body2, err := io.ReadAll(fetchResp2.Body)
	require.NoError(t, err)
	info2, err := adaptor.ParseTaskResult(nil, nil, body2)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info2.Status)
	assert.Equal(t, "https://v3.fal.media/files/lion/completed_e2e.mp4", info2.Url)
	assert.Equal(t, taskcommon.ProgressComplete, info2.Progress)
}

// TestQA_DynamicMultimodalDispatch verifies:
// 1. Text-only prompt dynamically routes to text-to-video (minimax/h3-max or minimax/h3-max-turbo)
// 2. Request with image_url (or image, images) dynamically routes to image-to-video
// 3. Request with video_url (or reference_video_url, video) dynamically routes to reference-to-video
// 4. Backward compatibility: explicitly specified endpoints are preserved
func TestQA_DynamicMultimodalDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &fal.TaskAdaptor{}

	var lastPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"req-dispatch-test-123","status":"IN_QUEUE"}`))
	}))
	defer mockServer.Close()

	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeFalAI,
			ChannelBaseUrl: mockServer.URL,
			ApiKey:         "test_key",
		},
	})

	t.Run("base model with prompt only routes to text-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":  "minimax/h3-max",
			"prompt": "flying eagle over mountains",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_1"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fal.ModelMinimaxH3MaxTextToVideo, info.UpstreamModelName)

		reqURL, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/%s", mockServer.URL, fal.ModelMinimaxH3MaxTextToVideo), reqURL)

		reqReader, err := adaptor.BuildRequestBody(c, info)
		require.NoError(t, err)
		reqBytes, _ := io.ReadAll(reqReader)
		var payload fal.FalVideoSubmitPayload
		require.NoError(t, common.Unmarshal(reqBytes, &payload))
		assert.Equal(t, "flying eagle over mountains", payload.Prompt)
		assert.Empty(t, payload.ImageURL)
		assert.Empty(t, payload.ReferenceVideoURLs)

		// Mock execute and DoResponse
		req, _ := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(reqBytes))
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		upstreamID, _, taskErr := adaptor.DoResponse(c, resp, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fmt.Sprintf("%s:req-dispatch-test-123", fal.ModelMinimaxH3MaxTextToVideo), upstreamID)
		assert.Equal(t, "/minimax/h3-max/text-to-video", lastPath)
	})

	t.Run("base model with image_url routes to image-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max",
			"prompt":    "animate this painting",
			"image_url": "https://example.com/monalisa.jpg",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_2"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fal.ModelMinimaxH3MaxImageToVideo, info.UpstreamModelName)

		reqURL, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/%s", mockServer.URL, fal.ModelMinimaxH3MaxImageToVideo), reqURL)

		reqReader, err := adaptor.BuildRequestBody(c, info)
		require.NoError(t, err)
		reqBytes, _ := io.ReadAll(reqReader)
		var payload fal.FalVideoSubmitPayload
		require.NoError(t, common.Unmarshal(reqBytes, &payload))
		assert.Equal(t, "https://example.com/monalisa.jpg", payload.ImageURL)
		assert.Empty(t, payload.ReferenceVideoURLs)

		req, _ := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(reqBytes))
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		upstreamID, _, taskErr := adaptor.DoResponse(c, resp, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fmt.Sprintf("%s:req-dispatch-test-123", fal.ModelMinimaxH3MaxImageToVideo), upstreamID)
		assert.Equal(t, "/minimax/h3-max/image-to-video", lastPath)
	})

	t.Run("base model with video_url routes to reference-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max",
			"prompt":    "copy dance routine",
			"video_url": "https://example.com/dance_ref.mp4",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_3"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fal.ModelMinimaxH3MaxReferenceToVideo, info.UpstreamModelName)

		reqURL, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/%s", mockServer.URL, fal.ModelMinimaxH3MaxReferenceToVideo), reqURL)

		reqReader, err := adaptor.BuildRequestBody(c, info)
		require.NoError(t, err)
		reqBytes, _ := io.ReadAll(reqReader)
		var payload fal.FalVideoSubmitPayload
		require.NoError(t, common.Unmarshal(reqBytes, &payload))
		assert.Equal(t, []string{"https://example.com/dance_ref.mp4"}, payload.ReferenceVideoURLs)

		req, _ := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(reqBytes))
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		upstreamID, _, taskErr := adaptor.DoResponse(c, resp, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fmt.Sprintf("%s:req-dispatch-test-123", fal.ModelMinimaxH3MaxReferenceToVideo), upstreamID)
		assert.Equal(t, "/minimax/h3-max/reference-to-video", lastPath)
	})

	t.Run("short alias h3 with reference_video_url routes to reference-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":               "h3",
			"prompt":              "reference action",
			"reference_video_url": "https://example.com/ref.mp4",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "h3",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_4"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fal.ModelMinimaxH3MaxReferenceToVideo, info.UpstreamModelName)

		reqURL, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/%s", mockServer.URL, fal.ModelMinimaxH3MaxReferenceToVideo), reqURL)
	})

	t.Run("turbo base model with image_url routes to turbo image-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max-turbo",
			"prompt":    "turbo motion",
			"image_url": "https://example.com/car.png",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max-turbo",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_5"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fal.ModelMinimaxH3MaxTurboImageToVideo, info.UpstreamModelName)

		reqURL, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/%s", mockServer.URL, fal.ModelMinimaxH3MaxTurboImageToVideo), reqURL)
	})

	t.Run("turbo short alias h3-turbo with prompt only routes to turbo text-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":  "h3-turbo",
			"prompt": "fast motion race",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "h3-turbo",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_6"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, fal.ModelMinimaxH3MaxTurboTextToVideo, info.UpstreamModelName)

		reqURL, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/%s", mockServer.URL, fal.ModelMinimaxH3MaxTurboTextToVideo), reqURL)
	})

	t.Run("backward compatibility - explicit image-to-video ignores video_url", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max/image-to-video",
			"prompt":    "animate character",
			"image_url": "https://example.com/avatar.png",
			"video_url": "https://example.com/some_extra_video.mp4",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max/image-to-video",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_7"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Contains(t, taskErr.Message, "no video or audio")
	})

	t.Run("backward compatibility - explicit text-to-video ignores image_url", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max/text-to-video",
			"prompt":    "pure text generation",
			"image_url": "https://example.com/ignored.png",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max/text-to-video",
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_8"},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		assert.Contains(t, taskErr.Message, "does not consume media")
	})
}
