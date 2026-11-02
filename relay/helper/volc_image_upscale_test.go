package helper

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 0, G: 255, B: 0, A: 255}}, image.Point{}, draw.Src)
	buf := new(bytes.Buffer)
	_ = png.Encode(buf, img)
	return buf.Bytes()
}

func TestGetAndValidOpenAIImageRequest_VolcImageUpscale_Rejections(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		relayMode   int
		contentType string
		body        string
		errContains string
	}{
		{
			name:        "rejected when not edits endpoint",
			relayMode:   relayconstant.RelayModeImagesGenerations,
			contentType: "application/json",
			body:        `{"model":"volc-image-upscale","image":"https://example.com/test.png"}`,
			errContains: "only supported on /v1/images/edits",
		},
		{
			name:        "rejected when multipart form data",
			relayMode:   relayconstant.RelayModeImagesEdits,
			contentType: "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
			body:        "------WebKitFormBoundary7MA4YWxkTrZu0gW\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nvolc-image-upscale\r\n------WebKitFormBoundary7MA4YWxkTrZu0gW--",
			errContains: "multipart/form-data is not supported for volc-image-upscale",
		},
		{
			name:        "rejected when image is missing",
			relayMode:   relayconstant.RelayModeImagesEdits,
			contentType: "application/json",
			body:        `{"model":"volc-image-upscale"}`,
			errContains: "image field is required",
		},
		{
			name:        "rejected when image is an array (multiple images)",
			relayMode:   relayconstant.RelayModeImagesEdits,
			contentType: "application/json",
			body:        `{"model":"volc-image-upscale","image":["https://example.com/1.png","https://example.com/2.png"]}`,
			errContains: "must be a single string URL",
		},
		{
			name:        "rejected when client attempts to override req_key in extra",
			relayMode:   relayconstant.RelayModeImagesEdits,
			contentType: "application/json",
			body:        `{"model":"volc-image-upscale","image":"https://example.com/1.png","extra_fields":{"req_key":"custom_hack"}}`,
			errContains: "req_key override is not allowed",
		},
		{
			name:        "rejected when n > 1",
			relayMode:   relayconstant.RelayModeImagesEdits,
			contentType: "application/json",
			body:        `{"model":"volc-image-upscale","image":"https://example.com/1.png","n":2}`,
			errContains: "n must be 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewBufferString(tc.body))
			c.Request.Header.Set("Content-Type", tc.contentType)

			_, err := GetAndValidOpenAIImageRequest(c, tc.relayMode)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestGetAndValidOpenAIImageRequest_VolcImageUpscale_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set up mock image server
	pngData := createTestPNG(200, 300)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData)
	}))
	defer server.Close()

	body := `{"model":"volc-image-upscale","image":"` + server.URL + `/input.png"}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// Use bypass SSRF context and custom client for mock server testing
	c.Request = c.Request.WithContext(service.WithBypassSSRFForTest(c.Request.Context()))
	c.Set("test_http_client", server.Client())

	req, err := GetAndValidOpenAIImageRequest(c, relayconstant.RelayModeImagesEdits)
	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, "volc-image-upscale", req.Model)
	assert.Equal(t, uint(1), *req.N)
}
