package jimeng

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequest_VolcImageUpscale(t *testing.T) {
	adaptor := &Adaptor{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "volc-image-upscale",
	}

	rawImage, _ := json.Marshal("https://example.com/source.png")
	req := dto.ImageRequest{
		Model: "volc-image-upscale",
		Image: rawImage,
	}

	converted, err := adaptor.ConvertImageRequest(c, info, req)
	require.NoError(t, err)

	payload, ok := converted.(imageRequestPayload)
	require.True(t, ok)
	assert.Equal(t, "lens_vida_nnsr", payload.ReqKey)
	assert.Equal(t, []string{"https://example.com/source.png"}, payload.ImageUrls)
	assert.True(t, payload.ReturnURL)
	assert.Empty(t, payload.Prompt)

	// Ensure client cannot override req_key or image_urls through ExtraFields
	reqWithTamper := dto.ImageRequest{
		Model:       "volc-image-upscale",
		Image:       rawImage,
		ExtraFields: json.RawMessage(`{"req_key":"malicious_key","image_urls":["https://evil.com/fake.png"]}`),
	}
	convertedTampered, err := adaptor.ConvertImageRequest(c, info, reqWithTamper)
	require.NoError(t, err)

	payloadTampered, ok := convertedTampered.(imageRequestPayload)
	require.True(t, ok)
	assert.Equal(t, "lens_vida_nnsr", payloadTampered.ReqKey)
	assert.Equal(t, []string{"https://example.com/source.png"}, payloadTampered.ImageUrls)
}

func TestSign_VolcengineOpenAPIV4(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://visual.volcengineapi.com/?Action=CVProcess&Version=2022-08-31", nil)

	body := []byte(`{"req_key":"lens_vida_nnsr","image_urls":["https://example.com/img.png"],"return_url":true}`)
	req, err := http.NewRequest(http.MethodPost, "https://visual.volcengineapi.com/?Action=CVProcess&Version=2022-08-31", bytes.NewReader(body))
	require.NoError(t, err)

	apiKey := "test_access_key|test_secret_key"
	err = Sign(c, req, apiKey)
	require.NoError(t, err)

	auth := req.Header.Get("Authorization")
	require.NotEmpty(t, auth)
	assert.True(t, strings.HasPrefix(auth, "HMAC-SHA256 Credential=test_access_key/"))
	assert.Contains(t, auth, "/cn-north-1/cv/request")
	assert.Contains(t, auth, "SignedHeaders=")
	assert.Contains(t, auth, "Signature=")

	assert.NotEmpty(t, req.Header.Get("X-Date"))
	assert.NotEmpty(t, req.Header.Get("X-Content-Sha256"))
	assert.Equal(t, "visual.volcengineapi.com", req.Header.Get("Host"))
}
