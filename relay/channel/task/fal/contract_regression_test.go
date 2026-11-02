package fal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResultAvailabilityAndHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		code       int
		body       string
		disconnect bool
		status     string
		reason     string
	}{
		{"empty completed", 200, `{"status":"COMPLETED"}`, false, string(model.TaskStatusInProgress), ""},
		{"empty result", 200, `{}`, false, string(model.TaskStatusInProgress), ""},
		{"not propagated", 404, `{}`, false, string(model.TaskStatusInProgress), ""},
		{"rate limited", 429, `{"detail":"slow down"}`, false, string(model.TaskStatusInProgress), ""},
		{"server retry", 503, `{"detail":[{"msg":"unavailable"}]}`, false, string(model.TaskStatusInProgress), ""},
		{"disconnect", 200, ``, true, string(model.TaskStatusInProgress), ""},
		{"invalid input", 422, `{"detail":[{"loc":["body","resolution"],"msg":"invalid resolution","type":"literal_error"}]}`, false, string(model.TaskStatusFailure), "body.resolution: invalid resolution (literal_error)"},
		{"video", 200, `{"video":{"url":"https://example.invalid/output.mp4"}}`, false, string(model.TaskStatusSuccess), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/minimax/h3-max/requests/task/status" {
					fmt.Fprint(w, `{"status":"COMPLETED"}`)
					return
				}
				if tc.disconnect {
					conn, _, err := w.(http.Hijacker).Hijack()
					require.NoError(t, err)
					conn.Close()
					return
				}
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			a := &TaskAdaptor{}
			resp, err := a.FetchTaskMap(server.URL, "fixture", map[string]any{"task_id": "minimax/h3-max/text-to-video:task"}, "")
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			result, err := a.ParseTaskResult(nil, nil, body)
			require.NoError(t, err)
			assert.Equal(t, tc.status, result.Status)
			assert.Equal(t, tc.reason, result.Reason)
			if tc.status != string(model.TaskStatusSuccess) {
				assert.Empty(t, result.Url)
			} else {
				assert.Equal(t, "https://example.invalid/output.mp4", result.Url)
			}
		})
	}
}

func TestCanonicalMediaRequestBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, model, op, endpoint string
		media                     map[string]any
		reject                    bool
	}{
		{"max first", "minimax/h3-max", "first_frame", ModelMinimaxH3MaxImageToVideo, map[string]any{"image_with_roles": []any{map[string]any{"url": "https://example.invalid/first.png", "role": "first_frame"}}}, false},
		{"turbo first", "minimax/h3-max-turbo", "first_frame", ModelMinimaxH3MaxTurboImageToVideo, map[string]any{"image_with_roles": []any{map[string]any{"url": "https://example.invalid/first.png", "role": "first_frame"}}}, false},
		{"ordered reference", "minimax/h3-max", "video_multi_ref", ModelMinimaxH3MaxReferenceToVideo, map[string]any{"image_urls": []string{"https://example.invalid/2.png", "https://example.invalid/1.png"}, "video_urls": []string{"https://example.invalid/2.mp4", "https://example.invalid/1.mp4"}}, false},
		{"turbo ref rejected", "minimax/h3-max-turbo", "video_multi_ref", "", map[string]any{"video_urls": []string{"https://example.invalid/1.mp4"}}, true},
		{"missing first", "minimax/h3-max", "first_frame", "", nil, true},
		{"unused media", "minimax/h3-max", "text_to_video", "", map[string]any{"image_urls": []string{"https://example.invalid/1.png"}}, true},
		{"unsupported role", "minimax/h3-max", "first_frame", "", map[string]any{"image_with_roles": []any{map[string]any{"url": "https://example.invalid/first.png", "role": "last_frame"}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sends := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends++
				assert.Equal(t, "/"+tc.endpoint, r.URL.Path)
				fmt.Fprint(w, `{}`)
			}))
			defer server.Close()
			raw := map[string]any{"model": tc.model, "operation": tc.op, "prompt": "fixture", "resolution": "768p"}
			for k, v := range tc.media {
				raw[k] = v
			}
			data, err := common.Marshal(raw)
			require.NoError(t, err)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(data))
			c.Request.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{OriginModelName: tc.model, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: server.URL}}
			a := &TaskAdaptor{}
			a.Init(info)
			taskErr := a.ValidateRequestAndSetAction(c, info)
			if tc.reject {
				require.NotNil(t, taskErr)
				assert.Equal(t, 0, sends)
				return
			}
			require.Nil(t, taskErr)
			address, err := a.BuildRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, server.URL+"/"+tc.endpoint, address)
			reader, err := a.BuildRequestBody(c, info)
			require.NoError(t, err)
			payload, err := io.ReadAll(reader)
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, common.Unmarshal(payload, &body))
			assert.Equal(t, "768P", body["resolution"])
			if tc.op == "video_multi_ref" {
				assert.Equal(t, []any{"https://example.invalid/2.png", "https://example.invalid/1.png"}, body["reference_image_urls"])
				assert.Equal(t, []any{"https://example.invalid/2.mp4", "https://example.invalid/1.mp4"}, body["reference_video_urls"])
				assert.NotContains(t, body, "video_url")
				assert.NotContains(t, body, "image_url")
			} else {
				assert.Equal(t, "https://example.invalid/first.png", body["image_url"])
			}
			resp, err := http.Post(address, "application/json", bytes.NewReader(payload))
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, 1, sends)
		})
	}
}
