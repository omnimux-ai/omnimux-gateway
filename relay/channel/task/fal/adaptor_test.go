package fal

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelMapping(t *testing.T) {
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("minimax/h3-max"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("h3-max"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("fal-ai/minimax/h3-max"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("minimax/h3"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("h3"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("minimax/h3-max/text-to-video"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("minimax/h3-max-t2v"))
	assert.Equal(t, ModelMinimaxH3MaxTextToVideo, GetFalEndpoint("h3-max-t2v"))

	assert.Equal(t, ModelMinimaxH3MaxImageToVideo, GetFalEndpoint("minimax/h3-max/image-to-video"))
	assert.Equal(t, ModelMinimaxH3MaxImageToVideo, GetFalEndpoint("minimax/h3-max-i2v"))
	assert.Equal(t, ModelMinimaxH3MaxImageToVideo, GetFalEndpoint("h3-max-image-to-video"))

	assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, GetFalEndpoint("minimax/h3-max/reference-to-video"))
	assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, GetFalEndpoint("minimax/h3-max-r2v"))
	assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, GetFalEndpoint("h3-max-reference-to-video"))

	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("minimax/h3-max-turbo"))
	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("h3-max-turbo"))
	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("minimax/h3-turbo"))
	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("h3-turbo"))
	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("minimax/h3-max-turbo/text-to-video"))
	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("minimax/h3-max-turbo-t2v"))
	assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, GetFalEndpoint("h3-max-turbo-t2v"))

	assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, GetFalEndpoint("minimax/h3-max-turbo/image-to-video"))
	assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, GetFalEndpoint("minimax/h3-max-turbo-i2v"))
	assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, GetFalEndpoint("h3-max-turbo-image-to-video"))

	assert.True(t, IsSupportedModel("minimax/h3-max"))
	assert.True(t, IsSupportedModel("h3-max"))
	assert.True(t, IsSupportedModel("minimax/h3"))
	assert.True(t, IsSupportedModel("h3"))
	assert.True(t, IsSupportedModel("minimax/h3-max/text-to-video"))
	assert.True(t, IsSupportedModel("minimax/h3-max/image-to-video"))
	assert.True(t, IsSupportedModel("minimax/h3-max/reference-to-video"))
	assert.True(t, IsSupportedModel("minimax/h3-max-turbo"))
	assert.True(t, IsSupportedModel("h3-max-turbo"))
	assert.True(t, IsSupportedModel("minimax/h3-turbo"))
	assert.True(t, IsSupportedModel("h3-turbo"))
	assert.True(t, IsSupportedModel("minimax/h3-max-turbo/text-to-video"))
	assert.True(t, IsSupportedModel("minimax/h3-max-turbo/image-to-video"))
}

func TestResolveFalEndpoint(t *testing.T) {
	t.Run("base h3-max dispatch", func(t *testing.T) {
		// prompt only -> text-to-video
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("minimax/h3-max", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("h3-max", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("minimax/h3", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("h3", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("fal-ai/minimax/h3-max", map[string]any{"prompt": "hello"}))

		// image_url -> image-to-video
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, ResolveFalEndpoint("minimax/h3-max", map[string]any{
			"prompt":    "hello",
			"image_url": "https://example.com/a.png",
		}))
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, ResolveFalEndpoint("h3-max", map[string]any{
			"prompt": "hello",
			"image":  "https://example.com/a.png",
		}))
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, ResolveFalEndpoint("minimax/h3", map[string]any{
			"prompt": "hello",
			"images": []any{"https://example.com/a.png"},
		}))

		// video_url -> reference-to-video
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, ResolveFalEndpoint("minimax/h3-max", map[string]any{
			"prompt":    "hello",
			"video_url": "https://example.com/v.mp4",
		}))
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, ResolveFalEndpoint("h3-max", map[string]any{
			"prompt":              "hello",
			"reference_video_url": "https://example.com/v.mp4",
		}))
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, ResolveFalEndpoint("minimax/h3", map[string]any{
			"prompt": "hello",
			"video":  "https://example.com/v.mp4",
		}))

		// video_url takes precedence over image_url for base model reference-to-video
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, ResolveFalEndpoint("minimax/h3-max", map[string]any{
			"prompt":    "hello",
			"video_url": "https://example.com/v.mp4",
			"image_url": "https://example.com/a.png",
		}))
	})

	t.Run("base turbo dispatch", func(t *testing.T) {
		// prompt only -> turbo text-to-video
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, ResolveFalEndpoint("minimax/h3-max-turbo", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, ResolveFalEndpoint("h3-max-turbo", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, ResolveFalEndpoint("minimax/h3-turbo", map[string]any{"prompt": "hello"}))
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, ResolveFalEndpoint("h3-turbo", map[string]any{"prompt": "hello"}))

		// image_url -> turbo image-to-video
		assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, ResolveFalEndpoint("minimax/h3-max-turbo", map[string]any{
			"prompt":    "hello",
			"image_url": "https://example.com/a.png",
		}))
		assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, ResolveFalEndpoint("h3-turbo", map[string]any{
			"prompt": "hello",
			"image":  "https://example.com/a.png",
		}))
	})

	t.Run("backward compatibility with explicit endpoints", func(t *testing.T) {
		// Explicit text-to-video stays text-to-video even if media is present
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("minimax/h3-max/text-to-video", map[string]any{
			"prompt":    "hello",
			"image_url": "https://example.com/a.png",
		}))
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, ResolveFalEndpoint("h3-max-t2v", map[string]any{
			"prompt": "hello",
		}))
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, ResolveFalEndpoint("minimax/h3-max-turbo/text-to-video", map[string]any{
			"prompt":    "hello",
			"video_url": "https://example.com/v.mp4",
		}))
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, ResolveFalEndpoint("h3-turbo-t2v", map[string]any{
			"prompt": "hello",
		}))

		// Explicit image-to-video stays image-to-video even if video_url is present
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, ResolveFalEndpoint("minimax/h3-max/image-to-video", map[string]any{
			"prompt":    "hello",
			"video_url": "https://example.com/v.mp4",
		}))
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, ResolveFalEndpoint("h3-max-i2v", map[string]any{
			"prompt": "hello",
		}))

		// Explicit reference-to-video stays reference-to-video
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, ResolveFalEndpoint("minimax/h3-max/reference-to-video", map[string]any{
			"prompt":    "hello",
			"image_url": "https://example.com/a.png",
		}))
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, ResolveFalEndpoint("h3-max-r2v", map[string]any{
			"prompt": "hello",
		}))

		// Explicit turbo image-to-video stays turbo image-to-video
		assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, ResolveFalEndpoint("minimax/h3-max-turbo/image-to-video", map[string]any{
			"prompt": "hello",
		}))
	})
}

func TestCompoundTaskID(t *testing.T) {
	composite := FormatUpstreamTaskID(ModelMinimaxH3Max, "018e6e5b-3b47-79f9-90b4-1773950b731d")
	assert.Equal(t, "minimax/h3-max:018e6e5b-3b47-79f9-90b4-1773950b731d", composite)

	endpoint, reqID, err := ParseUpstreamTaskID(composite)
	require.NoError(t, err)
	assert.Equal(t, ModelMinimaxH3Max, endpoint)
	assert.Equal(t, "018e6e5b-3b47-79f9-90b4-1773950b731d", reqID)

	assert.Equal(t, "single-id", FormatUpstreamTaskID("", "single-id"))

	// Fallback when no colon is present
	ep2, reqID2, err2 := ParseUpstreamTaskID("018e6e5b-single-id")
	require.NoError(t, err2)
	assert.Equal(t, "", ep2)
	assert.Equal(t, "018e6e5b-single-id", reqID2)

	// Empty ID returns error
	_, _, errEmpty := ParseUpstreamTaskID("")
	assert.Error(t, errEmpty)
}

func TestBuildSubmitPayload(t *testing.T) {
	t.Run("valid text-to-video", func(t *testing.T) {
		raw := map[string]any{
			"prompt":           "A futuristic cyberpunk cityscape at night",
			"duration":         10,
			"aspect_ratio":     "16:9",
			"resolution":       "1080p",
			"prompt_optimizer": false,
		}
		p, err := buildSubmitPayload(ModelMinimaxH3Max, raw)
		require.NoError(t, err)
		assert.Equal(t, "A futuristic cyberpunk cityscape at night", p.Prompt)
		assert.Equal(t, Duration10, p.Duration)
		assert.Equal(t, AspectRatio16x9, p.AspectRatio)
		assert.Equal(t, Resolution1080p, p.Resolution)
		assert.False(t, p.PromptOptimizer)
	})

	t.Run("missing prompt error", func(t *testing.T) {
		raw := map[string]any{
			"duration": 5,
		}
		_, err := buildSubmitPayload(ModelMinimaxH3Max, raw)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prompt is required")
	})

	t.Run("valid image-to-video with direct image_url", func(t *testing.T) {
		raw := map[string]any{
			"prompt":    "animate this character walking",
			"image_url": "https://example.com/character.png",
		}
		p, err := buildSubmitPayload(ModelMinimaxH3MaxImageToVideo, raw)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/character.png", p.ImageURL)
		assert.Equal(t, Duration5, p.Duration)
		assert.Equal(t, AspectRatio16x9, p.AspectRatio)
		assert.Equal(t, Resolution720p, p.Resolution)
		assert.True(t, p.PromptOptimizer)
	})

	t.Run("image-to-video with images array", func(t *testing.T) {
		raw := map[string]any{
			"prompt": "animate this character",
			"images": []any{"https://example.com/first.png"},
		}
		p, err := buildSubmitPayload(ModelMinimaxH3MaxTurboImageToVideo, raw)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/first.png", p.ImageURL)
	})

	t.Run("image-to-video missing image_url", func(t *testing.T) {
		raw := map[string]any{
			"prompt": "animate this character",
		}
		_, err := buildSubmitPayload(ModelMinimaxH3MaxImageToVideo, raw)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one image")
	})

	t.Run("reference-to-video with video_url", func(t *testing.T) {
		raw := map[string]any{
			"prompt":    "transfer motion",
			"video_url": "https://example.com/ref.mp4",
		}
		p, err := buildSubmitPayload(ModelMinimaxH3MaxReferenceToVideo, raw)
		require.NoError(t, err)
		assert.Equal(t, []string{"https://example.com/ref.mp4"}, p.ReferenceVideoURLs)
	})

	t.Run("reference-to-video missing video_url", func(t *testing.T) {
		raw := map[string]any{
			"prompt": "transfer motion",
		}
		_, err := buildSubmitPayload(ModelMinimaxH3MaxReferenceToVideo, raw)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires 1–12 references")
	})
}

func TestParseTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}

	t.Run("completed with video file", func(t *testing.T) {
		body := []byte(`{
			"video": {
				"url": "https://v3.fal.media/files/lion/output.mp4",
				"content_type": "video/mp4",
				"file_name": "output.mp4",
				"file_size": 5242880
			},
			"usage": {
				"execution_time": 8.42
			}
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
		assert.Equal(t, "https://v3.fal.media/files/lion/output.mp4", info.Url)
		assert.Equal(t, "100%", info.Progress)
	})

	t.Run("status response in queue", func(t *testing.T) {
		body := []byte(`{
			"status": "IN_QUEUE",
			"queue_position": 3,
			"response_url": "https://queue.fal.run/minimax/h3-max/requests/req-123"
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusQueued), info.Status)
		assert.Equal(t, "20%", info.Progress)
	})

	t.Run("status response in progress", func(t *testing.T) {
		body := []byte(`{
			"status": "IN_PROGRESS",
			"queue_position": 0,
			"logs": []
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
		assert.Equal(t, "30%", info.Progress)
	})

	t.Run("status response failed with error message", func(t *testing.T) {
		body := []byte(`{
			"status": "FAILED",
			"error": {
				"message": "NSFW content detected",
				"code": "content_moderation"
			}
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
		assert.Equal(t, "NSFW content detected", info.Reason)
	})

	t.Run("result response error detail", func(t *testing.T) {
		body := []byte(`{
			"detail": "Upstream resource exhausted"
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
		assert.Equal(t, "Upstream resource exhausted", info.Reason)
	})

	t.Run("result response detail object with message", func(t *testing.T) {
		body := []byte(`{
			"detail": {
				"message": "Quota exceeded"
			}
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
		assert.Equal(t, "Quota exceeded", info.Reason)
	})

	t.Run("status cancelled", func(t *testing.T) {
		body := []byte(`{
			"status": "CANCELLED"
		}`)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	})
}

func TestAdaptorRequestMethods(t *testing.T) {
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeFalAI,
			ChannelBaseUrl: "https://queue.fal.run",
			ApiKey:         "Key fal_key_secret_123",
		},
	})

	assert.Equal(t, "fal.ai", adaptor.GetChannelName())
	assert.Equal(t, 8, len(adaptor.GetModelList()))

	t.Run("build request url", func(t *testing.T) {
		u, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max-turbo",
		})
		require.NoError(t, err)
		assert.Equal(t, "https://queue.fal.run/minimax/h3-max-turbo/text-to-video", u)

		// With upstream model name
		u2, err2 := adaptor.BuildRequestURL(&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: "minimax/h3-max",
			},
			OriginModelName: "minimax/h3-max-turbo",
		})
		require.NoError(t, err2)
		assert.Equal(t, "https://queue.fal.run/minimax/h3-max/text-to-video", u2)
	})

	t.Run("build request header removes Key prefix cleanly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
		err := adaptor.BuildRequestHeader(nil, req, &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ApiKey: "Key secret_token_xyz",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Key secret_token_xyz", req.Header.Get("Authorization"))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	})

	t.Run("build request header removes Bearer prefix cleanly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
		err := adaptor.BuildRequestHeader(nil, req, &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ApiKey: "Bearer secret_token_xyz",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Key secret_token_xyz", req.Header.Get("Authorization"))
	})

	t.Run("do response parsing", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		respPayload := FalSubmitResponse{
			RequestID:   "018e6e5b-3b47-79f9-90b4-1773950b731d",
			Status:      "IN_QUEUE",
			ResponseURL: "https://queue.fal.run/minimax/h3-max/text-to-video/requests/018e6e5b",
			StatusURL:   "https://queue.fal.run/minimax/h3-max/text-to-video/requests/018e6e5b/status",
		}
		rawBytes, _ := common.Marshal(respPayload)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(rawBytes)),
		}

		info := &relaycommon.RelayInfo{
			TaskRelayInfo: &relaycommon.TaskRelayInfo{
				PublicTaskID: "omnimux_task_abc",
			},
			OriginModelName: "minimax/h3-max",
		}

		upstreamID, taskData, taskErr := adaptor.DoResponse(c, resp, info)
		assert.Nil(t, taskErr)
		assert.Equal(t, "minimax/h3-max/text-to-video:018e6e5b-3b47-79f9-90b4-1773950b731d", upstreamID)
		assert.NotEmpty(t, taskData)
		assert.Equal(t, 0, w.Body.Len(), "DoResponse must not write directly to client writer")
	})
}

func TestFetchTask_TwoPhase(t *testing.T) {
	t.Run("status IN_PROGRESS returns status response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/minimax/h3-max/requests/req-001/status" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"IN_PROGRESS","queue_position":0}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		adaptor := &TaskAdaptor{}
		resp, err := adaptor.FetchTaskMap(server.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-001",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
	})

	t.Run("status COMPLETED triggers second-phase fetch of final result", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/minimax/h3-max/requests/req-002/status":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
			case "/minimax/h3-max/requests/req-002":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"video": {
						"url": "https://v3.fal.media/files/lion/completed.mp4",
						"content_type": "video/mp4"
					}
				}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		adaptor := &TaskAdaptor{}
		resp, err := adaptor.FetchTaskMap(server.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-002",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
		assert.Equal(t, "https://v3.fal.media/files/lion/completed.mp4", info.Url)
	})

	t.Run("status COMPLETED but result returns 404 tolerates CDN lag", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/minimax/h3-max/requests/req-003/status":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
			case "/minimax/h3-max/requests/req-003":
				// CDN replication delay 404
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		adaptor := &TaskAdaptor{}
		resp, err := adaptor.FetchTaskMap(server.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-003",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
	})

	t.Run("status FAILED returns error detail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/minimax/h3-max/requests/req-004/status" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"FAILED","error":{"message":"safety check failed"}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		adaptor := &TaskAdaptor{}
		resp, err := adaptor.FetchTaskMap(server.URL, "dummy_key", map[string]any{
			"task_id": "minimax/h3-max:req-004",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
		assert.Equal(t, "safety check failed", info.Reason)
	})

	t.Run("empty task_id returns error", func(t *testing.T) {
		adaptor := &TaskAdaptor{}
		_, err := adaptor.FetchTaskMap("https://queue.fal.run", "key", map[string]any{}, "")
		assert.Error(t, err)
	})

	t.Run("status non-200 returns response directly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"unauthorized"}`))
		}))
		defer server.Close()

		adaptor := &TaskAdaptor{}
		resp, err := adaptor.FetchTaskMap(server.URL, "key", map[string]any{
			"task_id": "minimax/h3-max:req-500",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		info, err := adaptor.ParseTaskResult(nil, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusFailure), info.Status)
		assert.Equal(t, "unauthorized", info.Reason)
	})
}

func TestValidateRequestAndSetAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{}

	t.Run("valid request with model in info", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"prompt": "flying eagle",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:     &relaycommon.ChannelMeta{},
			TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
			OriginModelName: "minimax/h3-max",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		assert.Nil(t, taskErr)
		assert.Equal(t, constant.TaskActionGenerate, info.Action)
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, info.UpstreamModelName)
	})

	t.Run("valid request with model in raw body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":  "minimax/h3-max-turbo",
			"prompt": "futuristic car",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:   &relaycommon.ChannelMeta{},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		assert.Nil(t, taskErr)
		assert.Equal(t, "minimax/h3-max-turbo", info.OriginModelName)
		assert.Equal(t, ModelMinimaxH3MaxTurboTextToVideo, info.UpstreamModelName)
	})

	t.Run("missing model", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"prompt": "futuristic car",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:   &relaycommon.ChannelMeta{},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		assert.NotNil(t, taskErr)
	})

	t.Run("invalid payload missing prompt", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model": "minimax/h3-max",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			ChannelMeta:   &relaycommon.ChannelMeta{},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		assert.NotNil(t, taskErr)
	})

	t.Run("dynamic dispatch - text prompt routes to text-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":  "minimax/h3-max",
			"prompt": "futuristic city",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, info.UpstreamModelName)
		resolved, _ := c.Get("fal_resolved_endpoint")
		assert.Equal(t, ModelMinimaxH3MaxTextToVideo, resolved)

		url, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://queue.fal.run/"+ModelMinimaxH3MaxTextToVideo, url)
	})

	t.Run("dynamic dispatch - image_url routes to image-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max",
			"prompt":    "animate this image",
			"image_url": "https://example.com/photo.png",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, info.UpstreamModelName)
		resolved, _ := c.Get("fal_resolved_endpoint")
		assert.Equal(t, ModelMinimaxH3MaxImageToVideo, resolved)

		url, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://queue.fal.run/"+ModelMinimaxH3MaxImageToVideo, url)
	})

	t.Run("dynamic dispatch - video_url routes to reference-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":     "h3-max",
			"prompt":    "transfer motion from reference",
			"video_url": "https://example.com/dance.mp4",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "h3-max",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, info.UpstreamModelName)
		resolved, _ := c.Get("fal_resolved_endpoint")
		assert.Equal(t, ModelMinimaxH3MaxReferenceToVideo, resolved)

		url, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://queue.fal.run/"+ModelMinimaxH3MaxReferenceToVideo, url)
	})

	t.Run("dynamic dispatch - turbo with image routes to turbo image-to-video", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-turbo",
			"prompt":    "turbo animate",
			"image_url": "https://example.com/input.png",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-turbo",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)
		assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, info.UpstreamModelName)
		resolved, _ := c.Get("fal_resolved_endpoint")
		assert.Equal(t, ModelMinimaxH3MaxTurboImageToVideo, resolved)

		url, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://queue.fal.run/"+ModelMinimaxH3MaxTurboImageToVideo, url)
	})

	t.Run("backward compatibility - explicit endpoint ignored dynamic switch", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyBytes, _ := common.Marshal(map[string]any{
			"model":     "minimax/h3-max/image-to-video",
			"prompt":    "animate portrait",
			"image_url": "https://example.com/input.png",
			"video_url": "https://example.com/ignored_video.mp4",
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax/h3-max/image-to-video",
		}
		taskErr := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		_, retained := c.Get("task_request_raw")
		assert.False(t, retained)
	})
}

func TestBuildRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("task_request_raw", map[string]any{
		"prompt":       "sunset on beach",
		"duration":     10,
		"aspect_ratio": "9:16",
		"resolution":   "1080p",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "minimax/h3-max",
	}

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload FalVideoSubmitPayload
	err = common.Unmarshal(data, &payload)
	require.NoError(t, err)

	assert.Equal(t, "sunset on beach", payload.Prompt)
	assert.Equal(t, Duration10, payload.Duration)
	assert.Equal(t, AspectRatio9x16, payload.AspectRatio)
	assert.Equal(t, Resolution1080p, payload.Resolution)
}

func TestDoResponse_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{}

	t.Run("non-200 status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"detail":"bad request"}`))),
		}
		info := &relaycommon.RelayInfo{OriginModelName: "minimax/h3-max"}
		_, _, taskErr := adaptor.DoResponse(c, resp, info)
		assert.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	})

	t.Run("missing request_id in 200 response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"IN_QUEUE"}`))),
		}
		info := &relaycommon.RelayInfo{OriginModelName: "minimax/h3-max"}
		_, _, taskErr := adaptor.DoResponse(c, resp, info)
		assert.NotNil(t, taskErr)
		assert.Equal(t, http.StatusBadGateway, taskErr.StatusCode)
	})
}

func TestExtractionFallbacks(t *testing.T) {
	// Prompt alternatives: text, description
	assert.Equal(t, "text prompt", extractPrompt(map[string]any{"text": "text prompt"}))
	assert.Equal(t, "desc prompt", extractPrompt(map[string]any{"description": "desc prompt"}))

	// Image alternatives: image, input_reference, first_frame_image, images object
	assert.Equal(t, "https://example.com/img1.png", extractImageURL(map[string]any{"image": "https://example.com/img1.png"}))
	assert.Equal(t, "https://example.com/img2.png", extractImageURL(map[string]any{"input_reference": "https://example.com/img2.png"}))
	assert.Equal(t, "https://example.com/img3.png", extractImageURL(map[string]any{"first_frame_image": "https://example.com/img3.png"}))
	assert.Equal(t, "https://example.com/img4.png", extractImageURL(map[string]any{"images": []any{map[string]any{"url": "https://example.com/img4.png"}}}))

	// Video alternatives: reference_video_url, video
	assert.Equal(t, "https://example.com/v1.mp4", extractVideoURL(map[string]any{"reference_video_url": "https://example.com/v1.mp4"}))
	assert.Equal(t, "https://example.com/v2.mp4", extractVideoURL(map[string]any{"video": "https://example.com/v2.mp4"}))

	// Aspect ratio variations
	assert.Equal(t, AspectRatio1x1, extractAspectRatio(map[string]any{"aspect_ratio": "1:1"}))
	assert.Equal(t, AspectRatio4x3, extractAspectRatio(map[string]any{"aspect_ratio": "4:3"}))
	assert.Equal(t, AspectRatio21x9, extractAspectRatio(map[string]any{"aspect_ratio": "21:9"}))
	assert.Equal(t, AspectRatio16x9, extractAspectRatio(map[string]any{"aspect_ratio": "unknown"}))
}
