package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type missingArtifactAfterProbeDriver struct{ storage.StorageDriver }

func (driver missingArtifactAfterProbeDriver) Head(context.Context, string) (*storage.ObjectMetadata, error) {
	return &storage.ObjectMetadata{ContentType: "video/mp4", Size: 10}, nil
}
func (driver missingArtifactAfterProbeDriver) Get(context.Context, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, errors.New("object disappeared")
}
func TestStoredArtifactReadFailureReturnsErrorInsteadOfEmptySuccess(t *testing.T) {
	task := videoArtifactTestTask(t, "https://example.com/video.mp4")
	previous := storage.GetStorageDriver()
	storage.SetGlobalDriver(missingArtifactAfterProbeDriver{StorageDriver: previous})
	t.Cleanup(func() { storage.SetGlobalDriver(previous) })
	c, response := videoArtifactTestRequest(task, nil)
	c.Request.URL.RawQuery = "delivery=direct"
	TaskArtifactContent(c)
	require.NotEmpty(t, response.Body.String())
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, response.Header().Get("Location"))
}
