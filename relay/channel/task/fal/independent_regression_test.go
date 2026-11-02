package fal_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/fal"
	rc "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// submitIndependent exercises the public validation and serialization boundary.
func submitIndependent(t *testing.T, raw map[string]any) (map[string]any, bool) {
	t.Helper()
	b, err := common.Marshal(raw)
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewReader(b))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &rc.RelayInfo{OriginModelName: raw["model"].(string), ChannelMeta: &rc.ChannelMeta{ChannelBaseUrl: "http://127.0.0.1:1"}}
	a := &fal.TaskAdaptor{}
	a.Init(info)
	if e := a.ValidateRequestAndSetAction(c, info); e != nil {
		return nil, false
	}
	r, err := a.BuildRequestBody(c, info)
	if err != nil {
		return nil, false
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, common.Unmarshal(data, &out))
	return out, true
}

// These cases preserve Edward's independent QA expectations in the normal Go suite.
func TestIndependentArtifacts(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"bare completed", `{"status":"COMPLETED"}`, model.TaskStatusInProgress},
		{"malformed https artifact", `{"status":"COMPLETED","video":{"url":"https://"}}`, model.TaskStatusInProgress},
		{"failed stale artifact", `{"status":"FAILED","video":{"url":"https://example.invalid/stale.mp4"}}`, model.TaskStatusFailure},
		{"cancelled stale artifact", `{"status":"CANCELLED","video":{"url":"https://example.invalid/stale.mp4"}}`, model.TaskStatusFailure},
		{"array detail", `{"detail":[{"loc":["body",0],"msg":"invalid","type":"value_error"}]}`, model.TaskStatusFailure},
		{"valid artifact", `{"video":{"url":"https://example.invalid/out.mp4"}}`, model.TaskStatusSuccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := (&fal.TaskAdaptor{}).ParseTaskResult(nil, nil, []byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.Status)
			if tc.want != model.TaskStatusSuccess {
				assert.Empty(t, result.Url)
				assert.Empty(t, result.RemoteUrl)
			}
		})
	}
}

func TestIndependentPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, res := range []string{"720p", "720", "720P", " 720p "} {
		t.Run("reject legacy "+res, func(t *testing.T) {
			_, ok := submitIndependent(t, map[string]any{"model": "minimax/h3-max", "operation": "text_to_video", "prompt": "test", "resolution": res})
			assert.False(t, ok, "legacy 720 resolution silently accepted")
		})
	}
	for _, n := range []int{7001, 50000, 50001} {
		t.Run(fmt.Sprintf("prompt %d", n), func(t *testing.T) {
			_, ok := submitIndependent(t, map[string]any{"model": "minimax/h3-max", "prompt": strings.Repeat("字", n), "resolution": "768p"})
			assert.Equal(t, n <= 50000, ok)
		})
	}
	t.Run("three reference arrays ordered", func(t *testing.T) {
		raw := map[string]any{"model": "minimax/h3-max", "operation": "video_multi_ref", "prompt": "test", "resolution": "768p",
			"image_urls": []string{"https://example.invalid/i2.png", "https://example.invalid/i1.png"},
			"video_urls": []string{"https://example.invalid/v2.mp4", "https://example.invalid/v1.mp4"},
			"audio_urls": []string{"https://example.invalid/a2.mp3", "https://example.invalid/a1.mp3"}}
		out, ok := submitIndependent(t, raw)
		require.True(t, ok)
		for _, kind := range []string{"image", "video", "audio"} {
			expected, err := common.Marshal(raw[kind+"_urls"])
			require.NoError(t, err)
			actual, err := common.Marshal(out["reference_"+kind+"_urls"])
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		}
		assert.NotContains(t, out, "image_url")
		assert.NotContains(t, out, "video_url")
	})
	for _, n := range []int{9, 10} {
		t.Run(fmt.Sprintf("image count %d", n), func(t *testing.T) {
			images := make([]string, n)
			for i := range images {
				images[i] = "https://example.invalid/i.png"
			}
			_, ok := submitIndependent(t, map[string]any{"model": "minimax/h3-max", "operation": "video_multi_ref", "prompt": "test", "image_urls": images})
			assert.Equal(t, n == 9, ok)
		})
	}
}

func TestFailureAliasesAndArtifactURLs(t *testing.T) {
	for _, status := range []string{"FAILED", "FAILURE", "ERROR", "CANCELLED", "CANCELED", " failed "} {
		t.Run(status, func(t *testing.T) {
			body, err := common.Marshal(map[string]any{"status": status, "video": map[string]string{"url": "https://example.invalid/stale.mp4"}})
			require.NoError(t, err)
			result, err := (&fal.TaskAdaptor{}).ParseTaskResult(nil, nil, body)
			require.NoError(t, err)
			assert.Equal(t, model.TaskStatusFailure, result.Status)
			assert.NotEmpty(t, result.Reason)
			assert.Empty(t, result.Url)
		})
	}
	for _, address := range []string{"http://", "https://:443/video.mp4", "https:///video.mp4", "https://bad host/video.mp4", "https://example.invalid/%zz", "file:///video.mp4", ""} {
		t.Run("invalid "+address, func(t *testing.T) {
			body, err := common.Marshal(map[string]any{"status": "COMPLETED", "video": map[string]string{"url": address}})
			require.NoError(t, err)
			result, err := (&fal.TaskAdaptor{}).ParseTaskResult(nil, nil, body)
			require.NoError(t, err)
			assert.Equal(t, model.TaskStatusInProgress, result.Status)
			assert.Empty(t, result.Url)
		})
	}
	t.Run("normalized valid artifact", func(t *testing.T) {
		result, err := (&fal.TaskAdaptor{}).ParseTaskResult(nil, nil, []byte(`{"video":{"url":" HTTPS://example.invalid/out.mp4 "}}`))
		require.NoError(t, err)
		assert.Equal(t, model.TaskStatusSuccess, result.Status)
		assert.Equal(t, "HTTPS://example.invalid/out.mp4", result.Url)
	})
}
