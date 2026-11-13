package controller

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type uploadCountingBody struct {
	io.Reader
	consumed int
}

func (b *uploadCountingBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	b.consumed += n
	return n, e
}
func (b *uploadCountingBody) Close() error { return nil }

func TestFileUploadBodyLimitAliases(t *testing.T) {
	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		for _, kind := range []string{"base64", "stream", "url"} {
			for _, known := range []bool{true, false} {
				t.Run(prefix+"/"+kind+"/"+map[bool]string{true: "declared", false: "unknown"}[known], func(t *testing.T) {
					gin.SetMode(gin.TestMode)
					handler := FileUploadBase64
					payload := `{"base64_data":"` + strings.Repeat("A", 2048) + `"}`
					contentType := "text/plain"
					if kind == "url" {
						handler = FileUploadURL
						payload = `{"file_url":"https://example.invalid/` + strings.Repeat("a", 2048) + `"}`
					}
					if kind == "stream" {
						handler = FileUploadStream
						var buf bytes.Buffer
						w := multipart.NewWriter(&buf)
						f, e := w.CreateFormFile("file", "large.bin")
						require.NoError(t, e)
						_, e = f.Write(bytes.Repeat([]byte{1}, 2048))
						require.NoError(t, e)
						require.NoError(t, w.Close())
						payload = buf.String()
						contentType = w.FormDataContentType()
					}
					router := gin.New()
					router.POST(prefix+"/upload/"+kind, func(c *gin.Context) {
						c.Set("id", 42)
						// Exercise the shared production cap with a small budget for unknown-size streams.
						if !known && !limitFileUploadBody(c, 1024) {
							return
						}
						handler(c)
					})
					body := &uploadCountingBody{Reader: strings.NewReader(payload)}
					req := httptest.NewRequest(http.MethodPost, prefix+"/upload/"+kind, body)
					req.Header.Set("Content-Type", contentType)
					req.ContentLength = -1
					if known {
						req.ContentLength = 1 << 30
					}
					rec := httptest.NewRecorder()
					router.ServeHTTP(rec, req)
					require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
					if known {
						assert.Zero(t, body.consumed)
					} else {
						assert.LessOrEqual(t, body.consumed, 1025)
						assert.Positive(t, body.consumed)
					}
				})
			}
		}
	}
}

func TestFileUploadBase64DecodedBoundary(t *testing.T) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
		for _, length := range []int{1, 2, 3, 4, 5} {
			data := bytes.Repeat([]byte{255}, length)
			encoded := encoding.EncodeToString(data)
			// Preserve CR/LF and padding while enforcing exact decoded bytes.
			encoded = encoded[:2] + "\r\n" + encoded[2:]
			got, e := decodeFileUploadBase64(encoded, int64(length))
			require.NoError(t, e)
			assert.Equal(t, data, got)
			_, e = decodeFileUploadBase64(encoded, int64(length-1))
			assert.True(t, common.IsRequestBodyTooLargeError(e), "%v", e)
		}
	}
	_, e := decodeFileUploadBase64("%%%", 10)
	require.Error(t, e)
	assert.False(t, common.IsRequestBodyTooLargeError(e))
}

func TestFileUploadMultipartTemporaryFilesRemoved(t *testing.T) {
	for _, field := range []string{"file", "other"} {
		t.Run(field, func(t *testing.T) {
			router := setupFileTestEnv(t)
			router.MaxMultipartMemory = 1
			temp := t.TempDir()
			t.Setenv("TMPDIR", temp)
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			part, e := w.CreateFormFile(field, "data.bin")
			require.NoError(t, e)
			_, e = part.Write(bytes.Repeat([]byte{1}, 4096))
			require.NoError(t, e)
			require.NoError(t, w.Close())
			req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", &buf)
			req.Header.Set("Content-Type", w.FormDataContentType())
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			expected := http.StatusOK
			if field != "file" {
				expected = http.StatusBadRequest
			}
			require.Equal(t, expected, rec.Code, rec.Body.String())
			files, e := os.ReadDir(temp)
			require.NoError(t, e)
			assert.Empty(t, files)
		})
	}
}

func TestFileUploadCompressedBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, err := io.WriteString(zw, `{"base64_data":"`+strings.Repeat("A", 4096)+`"}`)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.Less(t, compressed.Len(), 1024)
	router := gin.New()
	router.Use(middleware.DecompressRequestMiddleware())
	router.POST("/v1/files/upload/base64", func(c *gin.Context) {
		c.Set("id", 42)
		if limitFileUploadBody(c, 1024) {
			FileUploadBase64(c)
		}
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/files/upload/base64", &compressed)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
}
