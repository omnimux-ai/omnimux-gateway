package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type directArtifactDriver struct {
	storage.StorageDriver
	gets  int
	signs int
}

func (d *directArtifactDriver) Get(context.Context, string) (io.ReadCloser, string, int64, error) {
	d.gets++
	return io.NopCloser(bytes.NewReader([]byte("image"))), "image/png", 5, nil
}
func (d *directArtifactDriver) IsR2() bool { return true }
func (d *directArtifactDriver) PresignGet(_ context.Context, key, contentType, disposition string, ttl time.Duration) (*storage.PresignResult, error) {
	d.signs++
	return &storage.PresignResult{URL: "https://account.r2.cloudflarestorage.com/files/" + key + "?signature=test", Method: http.MethodGet, ExpiresAt: time.Now().Add(ttl)}, nil
}

func TestStorageArtifactDirectDeliveryDoesNotReadBytes(t *testing.T) {
	driver := &directArtifactDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir())}
	previous := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previous) })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/t/artifacts/image/content?delivery=direct", nil)
	err := GetTaskArtifactStore().Serve(c, &model.Task{TaskID: "t"}, &StoredArtifactRef{ObjectKey: "task-artifacts/t/image", MimeType: "image/png", Size: 5})
	require.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Location"), "r2.cloudflarestorage.com")
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
	assert.Equal(t, 0, driver.gets)
	assert.Equal(t, 1, driver.signs)
}

func TestStorageArtifactCanonicalDeliveryKeepsBytes(t *testing.T) {
	driver := &directArtifactDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir())}
	previous := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previous) })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/t/artifacts/image/content", nil)
	require.NoError(t, GetTaskArtifactStore().Serve(c, &model.Task{TaskID: "t"}, &StoredArtifactRef{ObjectKey: "task-artifacts/t/image", MimeType: "image/png", Size: 5}))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "image", recorder.Body.String())
	assert.Equal(t, 0, driver.signs)
}
