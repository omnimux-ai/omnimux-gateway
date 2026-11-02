package sora

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIs720pResolution(t *testing.T) {
	tests := []struct {
		name     string
		req      relaycommon.TaskSubmitReq
		expected bool
	}{
		{
			name:     "Explicit 720p in Resolution",
			req:      relaycommon.TaskSubmitReq{Resolution: "720p"},
			expected: true,
		},
		{
			name:     "Explicit 720P uppercase",
			req:      relaycommon.TaskSubmitReq{Resolution: "720P"},
			expected: true,
		},
		{
			name:     "Explicit 480p in Resolution",
			req:      relaycommon.TaskSubmitReq{Resolution: "480p"},
			expected: false,
		},
		{
			name:     "Empty resolution",
			req:      relaycommon.TaskSubmitReq{},
			expected: false,
		},
		{
			name:     "720 in size (1280x720)",
			req:      relaycommon.TaskSubmitReq{Size: "1280x720"},
			expected: true,
		},
		{
			name:     "480 in size (640x480)",
			req:      relaycommon.TaskSubmitReq{Size: "640x480"},
			expected: false,
		},
		{
			name:     "Resolution in metadata",
			req:      relaycommon.TaskSubmitReq{Metadata: map[string]any{"resolution": "720p"}},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, is720pResolution(tc.req))
		})
	}
}

func TestResolveSeedanceUpstreamModel(t *testing.T) {
	// Seedance 2.5
	assert.Equal(t, "sd-2.5-480p-pro", resolveSeedanceUpstreamModel("sd-2.5-480p-pro", "seedance-2-5", false))
	assert.Equal(t, "sd-2.5-720p-pro", resolveSeedanceUpstreamModel("sd-2.5-480p-pro", "seedance-2-5", true))

	// Seedance 2.0
	assert.Equal(t, "seedace-2.0-480p", resolveSeedanceUpstreamModel("seedace-2.0-480p", "seedance-2-0", false))
	assert.Equal(t, "seedace-2.0-720p", resolveSeedanceUpstreamModel("seedace-2.0-480p", "seedance-2-0", true))

	// Unrelated model stays unchanged
	assert.Equal(t, "sora-2", resolveSeedanceUpstreamModel("sora-2", "sora-2", true))
}

func TestEstimateBillingResolutionMultiplier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{}

	// 1. Seedance 2.5 480p
	w480 := httptest.NewRecorder()
	c480, _ := gin.CreateTestContext(w480)
	c480.Set("task_request", relaycommon.TaskSubmitReq{
		Model:      "seedance-2-5",
		Resolution: "480p",
		Seconds:    "5",
	})
	info480 := &relaycommon.RelayInfo{OriginModelName: "seedance-2-5"}
	ratios480 := adaptor.EstimateBilling(c480, info480)
	assert.Equal(t, float64(5), ratios480["seconds"])
	_, hasRes480 := ratios480["resolution"]
	assert.False(t, hasRes480, "480p should not have extra resolution multiplier")

	// 2. Seedance 2.5 720p
	w720 := httptest.NewRecorder()
	c720, _ := gin.CreateTestContext(w720)
	c720.Set("task_request", relaycommon.TaskSubmitReq{
		Model:      "seedance-2-5",
		Resolution: "720p",
		Seconds:    "5",
	})
	info720 := &relaycommon.RelayInfo{OriginModelName: "seedance-2-5"}
	ratios720 := adaptor.EstimateBilling(c720, info720)
	assert.Equal(t, float64(5), ratios720["seconds"])
	assert.InDelta(t, 1.285714, ratios720["resolution"], 0.0001, "720p should apply 1.285714x resolution multiplier")

	// 3. Seedance 2.0 720p
	w20 := httptest.NewRecorder()
	c20, _ := gin.CreateTestContext(w20)
	c20.Set("task_request", relaycommon.TaskSubmitReq{
		Model:      "seedance-2-0",
		Resolution: "720p",
		Seconds:    "10",
	})
	info20 := &relaycommon.RelayInfo{OriginModelName: "seedance-2-0"}
	ratios20 := adaptor.EstimateBilling(c20, info20)
	assert.Equal(t, float64(10), ratios20["seconds"])
	assert.InDelta(t, 1.555556, ratios20["resolution"], 0.0001, "2.0 720p should apply 1.555556x resolution multiplier")
}

func TestBuildRequestBodySeedanceResolutionRouting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{}

	// Request for 720p
	payload := []byte(`{"model":"seedance-2-5","prompt":"a running cat","resolution":"720p","seconds":"5"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:      "seedance-2-5",
		Resolution: "720p",
		Seconds:    "5",
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "sd-2.5-480p-pro",
		},
	}

	reader, err := adaptor.BuildRequestBody(c, info)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(reader)
	assert.NoError(t, err)

	var reqBody map[string]any
	err = common.Unmarshal(bodyBytes, &reqBody)
	assert.NoError(t, err)
	assert.Equal(t, "sd-2.5-720p-pro", reqBody["model"])
	assert.Equal(t, "720p", reqBody["resolution"])
}
