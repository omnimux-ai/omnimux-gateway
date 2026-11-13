package storage

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageDriver_Operations(t *testing.T) {
	tempDir := t.TempDir()
	driver := NewLocalStorageDriver(tempDir)
	ctx := context.Background()

	key := "photos/test.txt"
	testContent := []byte("hello storage")

	// 1. Upload
	url, err := driver.Upload(ctx, key, testContent, "text/plain")
	require.NoError(t, err)
	assert.Empty(t, url) // No public base URL configured

	// 2. Get
	reader, mimeType, size, err := driver.Get(ctx, key)
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testContent, data)
	assert.Equal(t, int64(len(testContent)), size)
	assert.Equal(t, "application/octet-stream", mimeType)

	// 3. Delete
	err = driver.Delete(ctx, key)
	require.NoError(t, err)

	// 4. Verify deleted
	_, _, _, err = driver.Get(ctx, key)
	assert.Error(t, err)

	// 5. Path traversal protection
	_, err = driver.Upload(ctx, "../escape.txt", testContent, "text/plain")
	assert.Error(t, err)
}

func TestLocalStorageDriver_WithPublicURL(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.Setenv("FILE_PUBLIC_BASE_URL", "https://cdn.example.com")
	defer os.Unsetenv("FILE_PUBLIC_BASE_URL")

	driver := NewLocalStorageDriver(tempDir)
	assert.Equal(t, "https://cdn.example.com/photos/avatar.png", driver.GetPublicURL("photos/avatar.png"))
	assert.False(t, driver.IsR2())
}
