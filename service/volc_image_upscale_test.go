package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/bmp"
)

func createTestImage(width, height int, format string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 0, B: 0, A: 255}}, image.Point{}, draw.Src)
	buf := new(bytes.Buffer)
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		_ = jpeg.Encode(buf, img, nil)
	case "png":
		_ = png.Encode(buf, img)
	case "bmp":
		_ = bmp.Encode(buf, img)
	default:
		buf.WriteString("fake content")
	}
	return buf.Bytes()
}

func TestValidateVolcImageUpscaleURL_SSRFAndSchemeRejections(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		url         string
		errContains string
	}{
		{
			name:        "http scheme rejected",
			url:         "http://example.com/test.png",
			errContains: "only https is allowed",
		},
		{
			name:        "data url rejected",
			url:         "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
			errContains: "only https is allowed",
		},
		{
			name:        "file scheme rejected",
			url:         "file:///etc/passwd",
			errContains: "only https is allowed",
		},
		{
			name:        "userinfo rejected",
			url:         "https://user:pass@example.com/test.png",
			errContains: "userinfo not allowed",
		},
		{
			name:        "private loopback IP 127.0.0.1 rejected",
			url:         "https://127.0.0.1/test.png",
			errContains: "private or loopback IP not allowed",
		},
		{
			name:        "private IP 10.0.0.1 rejected",
			url:         "https://10.0.0.1/test.png",
			errContains: "private or loopback IP not allowed",
		},
		{
			name:        "private IP 192.168.1.1 rejected",
			url:         "https://192.168.1.1/test.png",
			errContains: "private or loopback IP not allowed",
		},
		{
			name:        "private link-local IP 169.254.169.254 rejected",
			url:         "https://169.254.169.254/test.png",
			errContains: "private or loopback IP not allowed",
		},
		{
			name:        "localhost rejected",
			url:         "https://localhost/test.png",
			errContains: "private or loopback IP not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateVolcImageUpscaleURL(ctx, tc.url, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestValidateVolcImageUpscaleURL_ImageValidation(t *testing.T) {
	ctx := WithBypassSSRFForTest(context.Background())

	// 1. Valid PNG within bounds
	pngData := createTestImage(100, 200, "png")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData)
	}))
	defer server.Close()

	meta, err := ValidateVolcImageUpscaleURL(ctx, server.URL+"/valid.png", server.Client())
	require.NoError(t, err)
	assert.Equal(t, 100, meta.Width)
	assert.Equal(t, 200, meta.Height)
	assert.Equal(t, "png", meta.Format)

	// 2. Valid BMP within bounds
	bmpData := createTestImage(50, 50, "bmp")
	bmpServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/bmp")
		_, _ = w.Write(bmpData)
	}))
	defer bmpServer.Close()

	meta, err = ValidateVolcImageUpscaleURL(ctx, bmpServer.URL+"/valid.bmp", bmpServer.Client())
	require.NoError(t, err)
	assert.Equal(t, 50, meta.Width)
	assert.Equal(t, 50, meta.Height)
	assert.Equal(t, "bmp", meta.Format)

	// 3. Valid JPEG within bounds
	jpegData := createTestImage(2128, 4064, "jpeg")
	jpegServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpegData)
	}))
	defer jpegServer.Close()

	meta, err = ValidateVolcImageUpscaleURL(ctx, jpegServer.URL+"/valid.jpg", jpegServer.Client())
	require.NoError(t, err)
	assert.Equal(t, 2128, meta.Width)
	assert.Equal(t, 4064, meta.Height)
	assert.Equal(t, "jpeg", meta.Format)

	// 4. Width out of bounds (too small < 50)
	tooSmallW := createTestImage(49, 100, "png")
	smallWServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tooSmallW)
	}))
	defer smallWServer.Close()
	_, err = ValidateVolcImageUpscaleURL(ctx, smallWServer.URL+"/small_w.png", smallWServer.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "width 49 out of bounds [50, 2128]")

	// 5. Width out of bounds (too large > 2128)
	tooLargeW := createTestImage(2129, 100, "png")
	largeWServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tooLargeW)
	}))
	defer largeWServer.Close()
	_, err = ValidateVolcImageUpscaleURL(ctx, largeWServer.URL+"/large_w.png", largeWServer.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "width 2129 out of bounds [50, 2128]")

	// 6. Height out of bounds (too small < 50)
	tooSmallH := createTestImage(100, 49, "png")
	smallHServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tooSmallH)
	}))
	defer smallHServer.Close()
	_, err = ValidateVolcImageUpscaleURL(ctx, smallHServer.URL+"/small_h.png", smallHServer.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "height 49 out of bounds [50, 4064]")

	// 7. Height out of bounds (too large > 4064)
	tooLargeH := createTestImage(100, 4065, "png")
	largeHServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tooLargeH)
	}))
	defer largeHServer.Close()
	_, err = ValidateVolcImageUpscaleURL(ctx, largeHServer.URL+"/large_h.png", largeHServer.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "height 4065 out of bounds [50, 4064]")

	// 8. Size exceeds 5 MB (5,242,880 bytes)
	largeData := make([]byte, VolcMaxImageSize+1)
	largeServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(largeData)))
		_, _ = w.Write(largeData)
	}))
	defer largeServer.Close()
	_, err = ValidateVolcImageUpscaleURL(ctx, largeServer.URL+"/large.png", largeServer.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 5MB limit")

	// 9. Unsupported format (e.g. plain text / non-image)
	badData := []byte("this is not an image")
	badServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(badData)
	}))
	defer badServer.Close()
	_, err = ValidateVolcImageUpscaleURL(ctx, badServer.URL+"/bad.txt", badServer.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported image format")
}
