package controller

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageThumbnailBackfillDefaultsToReadOnlyAndResumesPresetPages(t *testing.T) {
	setupInfluencerPresetTest(t)
	original := enqueueInfluencerPresetThumbnail
	calls := 0
	enqueueInfluencerPresetThumbnail = func(int64) bool { calls++; return true }
	t.Cleanup(func() { enqueueInfluencerPresetThumbnail = original })
	for _, name := range []string{"first", "second", "third"} {
		require.NoError(t, model.CreateInfluencerPreset(&model.InfluencerPreset{Name: name, Tier: "normal", Selection: "{}", PreviewKey: name, SheetKey: name}))
	}
	result, err := inspectImageThumbnailBatch(context.Background(), imageThumbnailBatchRequest{Scope: "presets", Limit: 2})
	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Equal(t, 0, calls)
	assert.Equal(t, int64(2), result.NextBefore)
	assert.Equal(t, int64(3), result.Items[0].ID)
	apply := false
	second, err := inspectImageThumbnailBatch(context.Background(), imageThumbnailBatchRequest{Scope: "presets", Before: result.NextBefore, Limit: 2, DryRun: &apply})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, int64(0), second.NextBefore)
	require.Len(t, second.Items, 1)
	assert.Equal(t, "queued", second.Items[0].Status)
}

func TestImageThumbnailBackfillDoesNotRestartExhaustedPresets(t *testing.T) {
	setupInfluencerPresetTest(t)
	require.NoError(t, model.CreateInfluencerPreset(&model.InfluencerPreset{Name: "expired", Tier: "normal", Selection: "{}", PreviewKey: "source", SheetKey: "source", ThumbnailAttempts: 3}))
	apply := false
	result, err := inspectImageThumbnailBatch(context.Background(), imageThumbnailBatchRequest{Scope: "presets", DryRun: &apply})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "failed", result.Items[0].Status)
	assert.Contains(t, result.Items[0].Reason, "exhausted")
}

func TestImageThumbnailBackfillTaskInspectsReadyDerivativeWithoutChangingTask(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	result, err := inspectImageThumbnailBatch(context.Background(), imageThumbnailBatchRequest{Scope: "tasks", Limit: 100})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "ready", result.Items[0].Status)
	assert.Equal(t, "image", result.Items[0].Key)
	assert.Equal(t, task.TaskID, result.Items[0].TaskID)
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, task.Status, stored.Status)
	assert.Equal(t, task.Quota, stored.Quota)
}

func TestImageThumbnailBackfillRejectsUnboundedAndUnknownScopes(t *testing.T) {
	for _, request := range []imageThumbnailBatchRequest{{Scope: "all"}, {Scope: "tasks", Limit: 101}, {Scope: "presets", Before: -1}} {
		_, err := inspectImageThumbnailBatch(context.Background(), request)
		assert.Error(t, err)
	}
}

func TestImageThumbnailBackfillKeepsThirdAttemptPendingUntilItFinishes(t *testing.T) {
	setupInfluencerPresetTest(t)
	preset := &model.InfluencerPreset{Name: "third running", Tier: "normal", Selection: "{}", PreviewKey: "source", SheetKey: "source", ThumbnailAttempts: 3, ThumbnailAttemptedAt: time.Now().UnixMilli()}
	require.NoError(t, model.CreateInfluencerPreset(preset))
	result, err := inspectImageThumbnailBatch(context.Background(), imageThumbnailBatchRequest{Scope: "presets"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "pending", result.Items[0].Status)
	require.NoError(t, model.DB.Model(preset).Updates(map[string]any{"thumbnail_key": "derived", "thumbnail_mime": "image/webp", "thumbnail_width": 640, "thumbnail_height": 1138, "thumbnail_attempted_at": 0}).Error)
	result, err = inspectImageThumbnailBatch(context.Background(), imageThumbnailBatchRequest{Scope: "presets"})
	require.NoError(t, err)
	assert.Equal(t, "ready", result.Items[0].Status)
}
