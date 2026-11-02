package jimeng

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJimengImageHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		StartTime:       time.Now(),
		OriginModelName: "volc-image-upscale",
		RelayMode:       relayconstant.RelayModeImagesEdits,
	}

	upstreamBody := `{"code":10000,"message":"Success","data":{"image_urls":["https://visual.volcengine.com/upscaled.png"],"request_id":"req-123"}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(upstreamBody)),
	}

	adaptor := &Adaptor{}
	usage, apiErr := adaptor.DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	assert.Equal(t, http.StatusOK, w.Code)
	var oaiResp dto.ImageResponse
	err := json.Unmarshal(w.Body.Bytes(), &oaiResp)
	require.NoError(t, err)
	require.Len(t, oaiResp.Data, 1)
	assert.Equal(t, "https://visual.volcengine.com/upscaled.png", oaiResp.Data[0].Url)
}

func TestJimengImageHandler_BusinessErrorNotMappedTo200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		StartTime:       time.Now(),
		OriginModelName: "volc-image-upscale",
		RelayMode:       relayconstant.RelayModeImagesEdits,
	}

	// Upstream returns HTTP 200 but business error code 50001
	upstreamBody := `{"code":50001,"message":"image algorithm internal failure","data":{}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(upstreamBody)),
	}

	adaptor := &Adaptor{}
	_, apiErr := adaptor.DoResponse(c, resp, info)
	require.NotNil(t, apiErr)
	assert.NotEqual(t, http.StatusOK, apiErr.StatusCode, "upstream business error must never map to HTTP 200")
	assert.Contains(t, apiErr.Error(), "image algorithm internal failure")
}

func TestJimengImageHandler_EmptyImageUrlsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		StartTime:       time.Now(),
		OriginModelName: "volc-image-upscale",
		RelayMode:       relayconstant.RelayModeImagesEdits,
	}

	// Upstream returns HTTP 200, code 10000, but empty image_urls
	upstreamBody := `{"code":10000,"message":"Success","data":{"image_urls":[],"binary_data_base64":[]}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(upstreamBody)),
	}

	adaptor := &Adaptor{}
	_, apiErr := adaptor.DoResponse(c, resp, info)
	require.NotNil(t, apiErr, "empty image output must be mapped to error, not success")
	assert.NotEqual(t, http.StatusOK, apiErr.StatusCode)
}

func TestJimengImageHandler_MalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		StartTime:       time.Now(),
		OriginModelName: "volc-image-upscale",
		RelayMode:       relayconstant.RelayModeImagesEdits,
	}

	upstreamBody := `not-a-json`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(upstreamBody)),
	}

	adaptor := &Adaptor{}
	_, apiErr := adaptor.DoResponse(c, resp, info)
	require.NotNil(t, apiErr)
	assert.NotEqual(t, http.StatusOK, apiErr.StatusCode)
}
