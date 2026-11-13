package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresignPutIsCreateOnly(t *testing.T) {
	driver, err := NewR2StorageDriver("https://account.r2.cloudflarestorage.com", "files", "test", "secret", "")
	require.NoError(t, err)
	ticket, err := driver.PresignPut(t.Context(), "videos/test.mp4", "video/mp4", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "*", ticket.Headers["If-None-Match"])
	parsed, err := url.Parse(ticket.URL)
	require.NoError(t, err)
	assert.Contains(t, parsed.Query().Get("X-Amz-SignedHeaders"), "if-none-match")
}

func TestR2PresignGetBindsResponseAndObject(t *testing.T) {
	driver, err := NewR2StorageDriver("https://account.r2.cloudflarestorage.com", "files", "test", "secret", "")
	require.NoError(t, err)
	ticket, err := driver.PresignGet(t.Context(), "task-artifacts/t/video", "video/mp4", `attachment; filename="video.mp4"`, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, ticket.Method)
	parsed, err := url.Parse(ticket.URL)
	require.NoError(t, err)
	assert.Equal(t, "/files/task-artifacts/t/video", parsed.Path)
	assert.Equal(t, "300", parsed.Query().Get("X-Amz-Expires"))
	assert.Equal(t, "private, no-store", parsed.Query().Get("response-cache-control"))
	assert.Equal(t, "video/mp4", parsed.Query().Get("response-content-type"))
	assert.Equal(t, `attachment; filename="video.mp4"`, parsed.Query().Get("response-content-disposition"))
	assert.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
}

func TestR2HeadDoesNotRequestObjectBody(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method)
		assert.Equal(t, "/files/videos/test.mp4", r.URL.Path)
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	driver, err := NewR2StorageDriver(server.URL, "files", "test", "secret", "")
	require.NoError(t, err)
	metadata, err := driver.Head(context.Background(), "videos/test.mp4")
	require.NoError(t, err)
	assert.Equal(t, int64(4096), metadata.Size)
	assert.Equal(t, "video/mp4", metadata.ContentType)
	assert.Equal(t, []string{http.MethodHead}, requests)
}
