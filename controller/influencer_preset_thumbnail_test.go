package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfluencerPresetThumbnailKeepsOriginalAssets(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	original := influencerPresetTestPNG(t, 940, 1672)
	driver.objects["original.png"] = influencerPresetTestObject{body: original, contentType: "image/png"}
	preset := &model.InfluencerPreset{Name: "character", Tier: "normal", Selection: "{}", PreviewKey: "original.png", PreviewMime: "image/png", PreviewWidth: 940, PreviewHeight: 1672, SheetKey: "original.png", SheetMime: "image/png", SourceTaskID: "source"}
	require.NoError(t, model.CreateInfluencerPreset(preset))
	previous := generateInfluencerPresetThumbnailImage
	t.Cleanup(func() { generateInfluencerPresetThumbnailImage = previous })
	generateInfluencerPresetThumbnailImage = func(_ context.Context, data []byte) (*service.ImageThumbnail, error) {
		require.Equal(t, original, data)
		return &service.ImageThumbnail{Data: []byte("derived-webp"), Width: 640, Height: 1138}, nil
	}
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	stored, err := model.GetInfluencerPreset(preset.Id)
	require.NoError(t, err)
	require.True(t, influencerPresetThumbnailReady(stored))
	assert.NotEqual(t, stored.PreviewKey, stored.ThumbnailKey)
	assert.Equal(t, original, driver.objects[stored.PreviewKey].body)
	projection := influencerPresetProjection(stored)
	assert.Equal(t, influencerPresetAssetPath(stored.Id, "preview"), projection["preview"].(gin.H)["path"])
	assert.Contains(t, projection["thumbnail"].(gin.H)["path"], "/asset/thumbnail?v=")
	engine := influencerPresetTestEngine(influencerPresetTestUser)
	thumb := httptest.NewRecorder()
	engine.ServeHTTP(thumb, httptest.NewRequest(http.MethodGet, influencerPresetAssetPath(stored.Id, "thumbnail"), nil))
	assert.Equal(t, http.StatusOK, thumb.Code)
	assert.Equal(t, "image/webp", thumb.Header().Get("Content-Type"))
	assert.Equal(t, "derived-webp", thumb.Body.String())
	deleted := httptest.NewRecorder()
	engine.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/api/influencer/presets/1", nil))
	assert.Equal(t, http.StatusOK, deleted.Code)
	assert.Empty(t, driver.objects)
}

func TestInfluencerPresetThumbnailFailurePreservesPublicationAndBoundsRetries(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	driver.objects["original.png"] = influencerPresetTestObject{body: influencerPresetTestPNG(t, 2, 3), contentType: "image/png"}
	preset := &model.InfluencerPreset{Name: "character", Tier: "normal", Selection: "{}", PreviewKey: "original.png", PreviewMime: "image/png", SheetKey: "original.png", SheetMime: "image/png"}
	require.NoError(t, model.CreateInfluencerPreset(preset))
	previous := generateInfluencerPresetThumbnailImage
	t.Cleanup(func() { generateInfluencerPresetThumbnailImage = previous })
	calls := 0
	generateInfluencerPresetThumbnailImage = func(context.Context, []byte) (*service.ImageThumbnail, error) {
		calls++
		return nil, errors.New("conversion failed")
	}
	for i := 0; i < 3; i++ {
		require.Error(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	}
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	assert.Equal(t, 3, calls)
	stored, err := model.GetInfluencerPreset(preset.Id)
	require.NoError(t, err)
	assert.False(t, influencerPresetThumbnailReady(stored))
	assert.False(t, influencerPresetThumbnailProcessing(stored))
	assert.Zero(t, stored.ThumbnailAttemptedAt)
	assert.NotContains(t, influencerPresetProjection(stored), "thumbnail")
	assert.Equal(t, "original.png", stored.PreviewKey)
	assert.Len(t, driver.objects, 1)
}

type influencerPresetThumbnailSourceChangeDriver struct {
	*influencerPresetTestDriver
	beforeUpload func()
}

func (d *influencerPresetThumbnailSourceChangeDriver) Upload(ctx context.Context, key string, data []byte, mime string) (string, error) {
	if d.beforeUpload != nil {
		d.beforeUpload()
	}
	return d.influencerPresetTestDriver.Upload(ctx, key, data, mime)
}

func TestInfluencerPresetThumbnailDiscardsStaleSourceUpload(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	driver.objects["original.png"] = influencerPresetTestObject{body: influencerPresetTestPNG(t, 2, 3), contentType: "image/png"}
	preset := &model.InfluencerPreset{Name: "character", Tier: "normal", Selection: "{}", PreviewKey: "original.png", PreviewMime: "image/png", SheetKey: "original.png", SheetMime: "image/png"}
	require.NoError(t, model.CreateInfluencerPreset(preset))
	previous := generateInfluencerPresetThumbnailImage
	t.Cleanup(func() { generateInfluencerPresetThumbnailImage = previous })
	generateInfluencerPresetThumbnailImage = func(context.Context, []byte) (*service.ImageThumbnail, error) {
		return &service.ImageThumbnail{Data: []byte("webp"), Width: 2, Height: 3}, nil
	}
	storage.SetGlobalDriver(&influencerPresetThumbnailSourceChangeDriver{influencerPresetTestDriver: driver, beforeUpload: func() {
		require.NoError(t, model.DB.Model(&model.InfluencerPreset{}).Where("id = ?", preset.Id).Update("preview_key", "replacement.png").Error)
	}})
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	stored, err := model.GetInfluencerPreset(preset.Id)
	require.NoError(t, err)
	assert.Empty(t, stored.ThumbnailKey)
	assert.Equal(t, "replacement.png", stored.PreviewKey)
	assert.Len(t, driver.deleted, 1)
	assert.Len(t, driver.objects, 1)
}

func TestInfluencerPresetThumbnailFinalRunningAttemptIsNotExhausted(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	driver.objects["original.png"] = influencerPresetTestObject{body: influencerPresetTestPNG(t, 2, 3), contentType: "image/png"}
	preset := &model.InfluencerPreset{Name: "character", Tier: "normal", Selection: "{}", PreviewKey: "original.png", PreviewMime: "image/png", SheetKey: "original.png", SheetMime: "image/png", ThumbnailAttempts: 2}
	require.NoError(t, model.CreateInfluencerPreset(preset))
	previous := generateInfluencerPresetThumbnailImage
	t.Cleanup(func() { generateInfluencerPresetThumbnailImage = previous })
	generateInfluencerPresetThumbnailImage = func(context.Context, []byte) (*service.ImageThumbnail, error) {
		running, err := model.GetInfluencerPreset(preset.Id)
		require.NoError(t, err)
		assert.Equal(t, 3, running.ThumbnailAttempts)
		assert.True(t, influencerPresetThumbnailProcessing(running))
		return &service.ImageThumbnail{Data: []byte("webp"), Width: 2, Height: 3}, nil
	}
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	stored, err := model.GetInfluencerPreset(preset.Id)
	require.NoError(t, err)
	assert.True(t, influencerPresetThumbnailReady(stored))
	assert.False(t, influencerPresetThumbnailProcessing(stored))
	assert.Zero(t, stored.ThumbnailAttemptedAt)
}

func TestInfluencerPresetThumbnailProcessingLeasePreventsDuplicateAndRecoversStaleAttempt(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	driver.objects["original.png"] = influencerPresetTestObject{body: influencerPresetTestPNG(t, 2, 3), contentType: "image/png"}
	preset := &model.InfluencerPreset{Name: "character", Tier: "normal", Selection: "{}", PreviewKey: "original.png", PreviewMime: "image/png", SheetKey: "original.png", SheetMime: "image/png", ThumbnailAttempts: 1, ThumbnailAttemptedAt: time.Now().UnixMilli()}
	require.NoError(t, model.CreateInfluencerPreset(preset))
	previous := generateInfluencerPresetThumbnailImage
	t.Cleanup(func() { generateInfluencerPresetThumbnailImage = previous })
	calls := 0
	generateInfluencerPresetThumbnailImage = func(context.Context, []byte) (*service.ImageThumbnail, error) {
		calls++
		return nil, errors.New("conversion failed")
	}
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	assert.Zero(t, calls)
	require.NoError(t, model.DB.Model(preset).Update("thumbnail_attempted_at", time.Now().Add(-31*time.Second).UnixMilli()).Error)
	require.Error(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	stored, err := model.GetInfluencerPreset(preset.Id)
	require.NoError(t, err)
	assert.Equal(t, 2, stored.ThumbnailAttempts)
	assert.Zero(t, stored.ThumbnailAttemptedAt)
	assert.False(t, influencerPresetThumbnailProcessing(stored))
	assert.Equal(t, 1, calls)
}
