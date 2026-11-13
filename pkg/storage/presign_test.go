package storage

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fork: r2-file-storage-presign — ticket shape and refusal paths.

func TestPresignPut_LocalDriverRefuses(t *testing.T) {
	driver := NewLocalStorageDriver(t.TempDir())

	ticket, err := driver.PresignPut(context.Background(), "videos/a.mp4", "video/mp4", time.Minute)
	assert.Nil(t, ticket)
	assert.ErrorIs(t, err, ErrPresignUnsupported)
}

func TestPresignPut_IssuesATicketWithoutAPublicOrigin(t *testing.T) {
	// Deployments without a public origin still accept direct uploads and publish
	// through the gateway download route, so the missing origin is not a refusal.
	driver, err := NewR2StorageDriver("https://account.r2.cloudflarestorage.com", "omnimux", "key", "secret", "")
	require.NoError(t, err)

	ticket, err := driver.PresignPut(context.Background(), "videos/a.mp4", "video/mp4", time.Minute)
	require.NoError(t, err)
	require.NotNil(t, ticket)
	assert.Equal(t, "PUT", ticket.Method)
	assert.Contains(t, ticket.URL, "/omnimux/videos/a.mp4")
	assert.Empty(t, driver.GetPublicURL("videos/a.mp4"), "no origin means the gateway serves the object")
}

func TestPresignPut_IssuesAPutTicketForTheObjectKey(t *testing.T) {
	const (
		endpoint  = "https://account.r2.cloudflarestorage.com"
		bucket    = "omnimux"
		accessKey = "test-access-key"
		ttl       = 15 * time.Minute
	)
	driver, err := NewR2StorageDriver(endpoint, bucket, accessKey, "secret", "https://cdn.example")
	require.NoError(t, err)

	before := time.Now()
	ticket, err := driver.PresignPut(context.Background(), "/videos/file_abc_clip.mp4", "video/mp4", ttl)
	require.NoError(t, err)

	assert.Equal(t, "PUT", ticket.Method)
	assert.Equal(t, "video/mp4", ticket.Headers["Content-Type"])

	parsed, err := url.Parse(ticket.URL)
	require.NoError(t, err)
	assert.Equal(t, "account.r2.cloudflarestorage.com", parsed.Host, "the ticket must point at storage, never at the gateway")
	assert.Equal(t, "/"+bucket+"/videos/file_abc_clip.mp4", parsed.Path)

	query := parsed.Query()
	assert.Equal(t, "AWS4-HMAC-SHA256", query.Get("X-Amz-Algorithm"))
	assert.NotEmpty(t, query.Get("X-Amz-Signature"))
	assert.Contains(t, query.Get("X-Amz-Credential"), accessKey)
	seconds, err := strconv.Atoi(query.Get("X-Amz-Expires"))
	require.NoError(t, err)
	assert.Equal(t, int(ttl.Seconds()), seconds)
	assert.Contains(t, query.Get("X-Amz-SignedHeaders"), "content-type")

	// The caller-visible expiry tracks the requested lifetime.
	assert.WithinDuration(t, before.Add(ttl), ticket.ExpiresAt, 5*time.Second)

	// The gateway still publishes the object at its own public origin.
	assert.Equal(t, "https://cdn.example/videos/file_abc_clip.mp4", driver.GetPublicURL("videos/file_abc_clip.mp4"))
}

func TestPresignPut_RejectsUnusableArguments(t *testing.T) {
	driver, err := NewR2StorageDriver("https://account.r2.cloudflarestorage.com", "omnimux", "key", "secret", "https://cdn.example")
	require.NoError(t, err)

	_, err = driver.PresignPut(context.Background(), "videos/a.mp4", "video/mp4", 0)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrPresignUnsupported), "a bad lifetime is a caller error, not a capability gap")

	_, err = driver.PresignPut(context.Background(), "/", "video/mp4", time.Minute)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "key"))
}
