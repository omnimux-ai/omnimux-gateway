package goeasy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUpstreamModel(t *testing.T) {
	cases := map[string]string{
		"gpt-image-2.5":       "gpt-image-2.5-async",
		"gpt-image-2.5-async": "gpt-image-2.5-async",
		"gpt-image-2":         "gpt-image-2.5-async",
		"image2-1k":           "gpt-image-2.5-async",
		"image2-1k-async":     "gpt-image-2.5-async",
		"gpt-image-2-async":   "gpt-image-2.5-async",
		"gpt-image-2.5-hd":    "image2.5-pro-async",
		"image2.5-pro":        "image2.5-pro-async",
		"image2.5-pro-async":  "image2.5-pro-async",
		"image2-pro":          "image2.5-pro-async",
		"image2-2k4k":         "image2.5-pro-async",
		"image2-2k4k-async":   "image2.5-pro-async",
		"image2-pro-async":    "image2.5-pro-async",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeUpstreamModel(in), in)
	}
}

func TestParseQueryToTaskInfoSuccess(t *testing.T) {
	body := []byte(`{
		"items": [{
			"id": "task_abc",
			"status": "success",
			"data": [{"url": "https://img.example.com/a.png", "b64_json": "", "revised_prompt": "x"}],
			"error": "",
			"progress": "receiving_image"
		}],
		"missing_ids": []
	}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://img.example.com/a.png", info.Url)
	assert.Equal(t, "https://img.example.com/a.png", info.RemoteUrl)
}

func TestParseQueryToTaskInfoProcessing(t *testing.T) {
	body := []byte(`{
		"items": [{
			"id": "task_abc",
			"status": "processing",
			"progress": "image_poll_wait",
			"data": [],
			"error": ""
		}],
		"missing_ids": []
	}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
	assert.Equal(t, "image_poll_wait", info.Progress)
}

func TestParseQueryToTaskInfoError(t *testing.T) {
	body := []byte(`{
		"items": [{
			"id": "task_abc",
			"status": "error",
			"error": "upstream failed",
			"data": []
		}],
		"missing_ids": []
	}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	assert.Equal(t, "upstream failed", info.Reason)
}

func TestParseQueryMissing(t *testing.T) {
	body := []byte(`{"items":[],"missing_ids":["task_x"]}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, "FAILURE", info.Status)
}

func TestBuildRequestBodyAdaptiveDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a := &TaskAdaptor{}

	testCase := func(bodyReq string, wantModel string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(bodyReq))
		c.Request.Header.Set("Content-Type", "application/json")
		var req relaycommon.TaskSubmitReq
		_ = common.Unmarshal([]byte(bodyReq), &req)
		c.Set("task_request", req)

		var raw map[string]any
		if err := common.Unmarshal([]byte(bodyReq), &raw); err == nil {
			if q, ok := raw["quality"].(string); ok {
				c.Set("task_quality", q)
			}
		}

		body, err := a.BuildRequestBody(c, &relaycommon.RelayInfo{OriginModelName: "gpt-image-2.5"})
		require.NoError(t, err)
		var parsed map[string]any
		readBytes, _ := io.ReadAll(body)
		require.NoError(t, common.Unmarshal(readBytes, &parsed))
		assert.Equal(t, wantModel, parsed["model"])
	}

	testCase(`{"prompt":"test","size":"1024x1024"}`, "gpt-image-2.5-async")
	testCase(`{"prompt":"test","size":"2048x2048"}`, "image2.5-pro-async")
	testCase(`{"prompt":"test","quality":"hd"}`, "image2.5-pro-async")
	testCase(`{"prompt":"test","quality":"pro"}`, "image2.5-pro-async")
}

func TestEstimateBillingResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a := &TaskAdaptor{}

	testUnits := func(bodyReq string, wantUnits float64) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(bodyReq))
		c.Request.Header.Set("Content-Type", "application/json")
		var req relaycommon.TaskSubmitReq
		_ = common.Unmarshal([]byte(bodyReq), &req)
		c.Set("task_request", req)

		var raw map[string]any
		if err := common.Unmarshal([]byte(bodyReq), &raw); err == nil {
			if q, ok := raw["quality"].(string); ok {
				c.Set("task_quality", q)
			}
		}

		billing := a.EstimateBilling(c, &relaycommon.RelayInfo{OriginModelName: "gpt-image-2.5"})
		assert.Equal(t, wantUnits, billing["units"])
	}

	testUnits(`{"prompt":"test","size":"1024x1024"}`, 1.0)
	testUnits(`{"prompt":"test","size":"2048x2048"}`, 3.0)
	testUnits(`{"prompt":"test","quality":"hd"}`, 3.0)
}

// Exercise the actual HTTP sender so a ratio left in a prepared request cannot
// pass only because a local task was accepted.
func TestProviderImageSizeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, size, quality, pixels, model string
		units                              float64
	}{
		{"square", "1:1", "auto", "1024x1024", "gpt-image-2.5-async", 1},
		{"portrait", "2:3", "auto", "1024x1536", "gpt-image-2.5-async", 1},
		{"landscape", "3:2", "auto", "1536x1024", "gpt-image-2.5-async", 1},
		{"portrait-four", "3:4", "auto", "1024x1365", "gpt-image-2.5-async", 1},
		{"landscape-four", "4:3", "auto", "1365x1024", "gpt-image-2.5-async", 1},
		{"vertical", "9:16", "auto", "1088x1920", "gpt-image-2.5-async", 1},
		{"wide", " 16:9 ", "auto", "1920x1088", "gpt-image-2.5-async", 1},
		{"pixels", "1536x1024", "auto", "1536x1024", "gpt-image-2.5-async", 1},
		{"unicode-pixels", " 1920×1088 ", "auto", "1920x1088", "gpt-image-2.5-async", 1},
		{"uppercase-pixels", "1088X1920", "auto", "1088x1920", "gpt-image-2.5-async", 1},
		{"explicit-2k", "2048×2048", "auto", "2048x2048", "image2.5-pro-async", 3},
		{"explicit-4k", "4096x4096", "auto", "4096x4096", "image2.5-pro-async", 3},
		{"high-quality", "16:9", "high", "1920x1088", "image2.5-pro-async", 3},
		{"hd-quality", "16:9", "hd", "1920x1088", "image2.5-pro-async", 3},
		{"pro-quality", "16:9", "pro", "1920x1088", "image2.5-pro-async", 3},
	} {
		for _, edit := range []bool{false, true} {
			mode := "generate"
			if edit {
				mode = "edit"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				var captured map[string]any
				var capturedPath string
				var decodeErr error
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedPath = r.URL.Path
					decodeErr = common.DecodeJson(r.Body, &captured)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"id":"local-task","status":"queued"}`)
				}))
				defer upstream.Close()
				path := "/v1/images/generations"
				payload := map[string]any{"model": "gpt-image-2.5", "prompt": "same character in several views", "size": tc.size, "quality": tc.quality, "metadata": map[string]any{"seed": 123}}
				if edit {
					path = "/v1/images/edits"
					payload["images"] = []string{"https://images.example/front.png", "https://images.example/side.png"}
				}
				encoded, err := common.Marshal(payload)
				require.NoError(t, err)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(encoded)))
				c.Request.Header.Set("Content-Type", "application/json")
				info := &relaycommon.RelayInfo{OriginModelName: "gpt-image-2.5", ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: upstream.URL, ApiKey: "local-placeholder"}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
				a := &TaskAdaptor{}
				a.Init(info)
				require.Nil(t, a.ValidateRequestAndSetAction(c, info))
				before, err := relaycommon.GetTaskRequest(c)
				require.NoError(t, err)
				beforeJSON, err := common.Marshal(before)
				require.NoError(t, err)
				assert.Equal(t, tc.units, a.EstimateBilling(c, info)["units"])
				body, err := a.BuildRequestBody(c, info)
				require.NoError(t, err)
				response, err := a.DoRequest(c, info, body)
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
				require.NoError(t, decodeErr)
				assert.Equal(t, tc.pixels, captured["size"])
				assert.Equal(t, tc.model, captured["model"])
				assert.Equal(t, tc.quality, captured["quality"])
				assert.Equal(t, payload["prompt"], captured["prompt"])
				if edit {
					assert.Equal(t, "/api/image-tasks/edits", capturedPath)
					assert.Equal(t, []any{"https://images.example/front.png", "https://images.example/side.png"}, captured["images"])
				} else {
					assert.Equal(t, "/api/image-tasks/generations", capturedPath)
					assert.NotContains(t, captured, "images")
				}
				after, err := relaycommon.GetTaskRequest(c)
				require.NoError(t, err)
				afterJSON, err := common.Marshal(after)
				require.NoError(t, err)
				assert.Equal(t, beforeJSON, afterJSON, "supplier preparation must not mutate routing/replay metadata")
				assert.Equal(t, tc.units, a.EstimateBilling(c, info)["units"])
			})
		}
	}
}
