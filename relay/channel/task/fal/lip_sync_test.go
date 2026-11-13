package fal_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/task/fal"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLipSyncModelMapping(t *testing.T) {
	aliases := []string{
		"minimax-h3-lip-sync",
		"minimax/h3-max/lip-sync/image-to-video",
		"minimax/h3-max/lip-sync",
		"h3-max-lip-sync",
		"h3-lip-sync",
		"minimax/h3-lip-sync",
	}

	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			ep := fal.GetFalEndpoint(alias)
			assert.Equal(t, fal.ModelMinimaxH3MaxLipSyncImageToVideo, ep)
			assert.True(t, fal.IsSupportedModel(alias))
		})
	}
}

func TestLipSyncAdaptorFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &fal.TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeFalAI,
			ChannelBaseUrl: "https://queue.fal.run",
			ApiKey:         "Key test_key_123",
		},
	})

	t.Run("valid lip-sync request without prompt", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyJSON := `{
			"model": "minimax-h3-lip-sync",
			"image_url": "https://example.com/face.jpg",
			"audio_url": "https://example.com/speech.mp3",
			"resolution": "1080P",
			"enable_transcription": true,
			"seed": 42
		}`
		c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewBufferString(bodyJSON))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax-h3-lip-sync",
		}

		err := adaptor.ValidateRequestAndSetAction(c, info)
		require.Nil(t, err)
		assert.Equal(t, fal.ModelMinimaxH3MaxLipSyncImageToVideo, info.UpstreamModelName)

		// Build URL
		url, urlErr := adaptor.BuildRequestURL(info)
		require.NoError(t, urlErr)
		assert.Equal(t, "https://queue.fal.run/minimax/h3-max/lip-sync/image-to-video", url)

		// Build Request Body
		reader, bodyErr := adaptor.BuildRequestBody(c, info)
		require.NoError(t, bodyErr)
		reqBytes, _ := io.ReadAll(reader)

		var parsed map[string]any
		require.NoError(t, common.Unmarshal(reqBytes, &parsed))
		assert.Equal(t, "https://example.com/face.jpg", parsed["image_url"])
		assert.Equal(t, "https://example.com/speech.mp3", parsed["audio_url"])
		assert.Equal(t, "1080P", parsed["resolution"])
		assert.Equal(t, true, parsed["enable_transcription"])
		assert.Nil(t, parsed["prompt"])
	})

	t.Run("missing image fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyJSON := `{
			"model": "minimax-h3-lip-sync",
			"audio_url": "https://example.com/speech.mp3"
		}`
		c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewBufferString(bodyJSON))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax-h3-lip-sync",
		}

		err := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, err)
		assert.Contains(t, err.Message, "image_url is required")
	})

	t.Run("missing audio fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bodyJSON := `{
			"model": "minimax-h3-lip-sync",
			"image_url": "https://example.com/face.jpg"
		}`
		c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewBufferString(bodyJSON))
		c.Request.Header.Set("Content-Type", "application/json")

		info := &relaycommon.RelayInfo{
			OriginModelName: "minimax-h3-lip-sync",
		}

		err := adaptor.ValidateRequestAndSetAction(c, info)
		require.NotNil(t, err)
		assert.Contains(t, err.Message, "audio_url is required")
	})

	t.Run("polling URL cleans lip-sync path", func(t *testing.T) {
		var requestedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"IN_QUEUE"}`))
		}))
		defer server.Close()

		resp, err := adaptor.FetchTaskMap(server.URL, "test_key", map[string]any{
			"task_id": "minimax/h3-max/lip-sync/image-to-video:req_abc123",
		}, "")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "/minimax/h3-max/requests/req_abc123/status", requestedPath)
	})
}
