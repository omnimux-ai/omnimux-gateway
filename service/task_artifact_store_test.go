package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageArtifactStoreRoundTrip(t *testing.T) {
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
	t.Cleanup(storage.ResetGlobalDriver)

	store := GetTaskArtifactStore()
	require.NotNil(t, store)
	assert.True(t, store.Enabled())

	task := &model.Task{TaskID: "task-roundtrip-1"}

	ref, err := store.Resolve(task, "image")
	require.NoError(t, err)
	assert.Nil(t, ref, "unpersisted artifact must resolve to nil")

	payload := []byte("fake-image-bytes")
	ref, err = store.Persist(t.Context(), task, types.TaskArtifact{
		Key: "image", Type: "image", MimeType: "image/png",
	}, strings.NewReader(string(payload)))
	require.NoError(t, err)
	require.NotNil(t, ref)
	assert.Equal(t, TaskArtifactObjectPrefix+"/task-roundtrip-1/image", ref.ObjectKey)
	assert.Equal(t, "image/png", ref.MimeType)
	assert.Equal(t, int64(len(payload)), ref.Size)
	assert.Equal(t, "local", ref.Backend)

	resolved, err := store.Resolve(task, "image")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, ref.ObjectKey, resolved.ObjectKey)
}

func TestStorageArtifactStoreSanitizesUnsafeMimeTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
	t.Cleanup(storage.ResetGlobalDriver)

	store := GetTaskArtifactStore()
	task := &model.Task{TaskID: "task-mime-1"}
	ref, err := store.Persist(t.Context(), task, types.TaskArtifact{
		Key: "page", Type: "file", MimeType: "text/html",
	}, strings.NewReader("<script>alert(1)</script>"))
	require.NoError(t, err)
	require.NotNil(t, ref)
	assert.Equal(t, "application/octet-stream", ref.MimeType)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, store.Serve(c, task, ref))
	assert.Equal(t, "application/octet-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "attachment", strings.SplitN(recorder.Header().Get("Content-Disposition"), ";", 2)[0])
	assert.Equal(t, "sandbox; default-src 'none'", recorder.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))

	safeRef, err := store.Persist(t.Context(), task, types.TaskArtifact{
		Key: "image", Type: "image", MimeType: "image/png",
	}, strings.NewReader("png"))
	require.NoError(t, err)
	require.NotNil(t, safeRef)
	assert.Equal(t, "image/png", safeRef.MimeType)
}

func TestStorageArtifactStoreRejectsUnsafeKeys(t *testing.T) {
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
	t.Cleanup(storage.ResetGlobalDriver)

	store := GetTaskArtifactStore()
	task := &model.Task{TaskID: "../escape"}
	ref, err := store.Resolve(task, "image")
	require.NoError(t, err)
	assert.Nil(t, ref)

	_, err = store.Persist(t.Context(), task, types.TaskArtifact{Key: "image", Type: "image"}, strings.NewReader("x"))
	assert.ErrorIs(t, err, ErrTaskArtifactStoreDisabled)

	_, err = store.Persist(t.Context(), &model.Task{TaskID: "ok"}, types.TaskArtifact{Key: "..", Type: "file"}, strings.NewReader("x"))
	assert.ErrorIs(t, err, ErrTaskArtifactStoreDisabled)
}

var _ TaskArtifactStore = storageArtifactStore{}
