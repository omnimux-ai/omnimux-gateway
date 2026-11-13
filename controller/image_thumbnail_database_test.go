package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageThumbnailDatabaseMigrationAndLease(t *testing.T) {
	db, dialect := openTaskDialectDatabase(t, &model.InfluencerPreset{})
	previousDB := model.DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousDriver := storage.GetStorageDriver()
	previousGenerate := generateInfluencerPresetThumbnailImage
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	driver := newInfluencerPresetTestDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMain, previousLog)
		storage.SetGlobalDriver(previousDriver)
		generateInfluencerPresetThumbnailImage = previousGenerate
	})

	// Start with an actual pre-feature table containing a published card, then
	// run the same additive migration the application uses at startup.
	columns := []string{"ThumbnailKey", "ThumbnailMime", "ThumbnailWidth", "ThumbnailHeight", "ThumbnailAttempts", "ThumbnailAttemptedAt"}
	for _, column := range columns {
		require.NoError(t, db.Migrator().DropColumn(&model.InfluencerPreset{}, column))
	}
	require.NoError(t, db.Model(&model.InfluencerPreset{}).Create(map[string]interface{}{
		"name": "existing card", "tier": "normal", "selection": "{}", "preview_key": "original.png", "preview_mime": "image/png", "preview_width": 940, "preview_height": 1672,
		"sheet_key": "sheet.png", "sheet_mime": "image/png", "sheet_width": 1880, "sheet_height": 1672, "source_task_id": "legacy-source",
	}).Error)
	require.NoError(t, db.AutoMigrate(&model.InfluencerPreset{}))
	for _, column := range columns {
		assert.True(t, db.Migrator().HasColumn(&model.InfluencerPreset{}, column))
	}
	var preset model.InfluencerPreset
	require.NoError(t, db.Where("source_task_id = ?", "legacy-source").First(&preset).Error)
	assert.Equal(t, "original.png", preset.PreviewKey)
	assert.Equal(t, "sheet.png", preset.SheetKey)
	assert.False(t, influencerPresetThumbnailReady(&preset))
	var nullAttempts int64
	require.NoError(t, db.Model(&model.InfluencerPreset{}).Where("id = ? AND thumbnail_attempts IS NULL AND thumbnail_attempted_at IS NULL", preset.Id).Count(&nullAttempts).Error)
	require.Equal(t, int64(1), nullAttempts, "migrated rows must exercise legacy NULL counters")

	original := influencerPresetTestPNG(t, 2, 3)
	driver.objects[preset.PreviewKey] = influencerPresetTestObject{body: original, contentType: "image/png"}
	calls := 0
	generateInfluencerPresetThumbnailImage = func(_ context.Context, data []byte) (*service.ImageThumbnail, error) {
		calls++
		assert.Equal(t, original, data)
		var live model.InfluencerPreset
		require.NoError(t, db.First(&live, preset.Id).Error)
		assert.Equal(t, 1, live.ThumbnailAttempts, "NULL counter is claimed once before encoding")
		require.Positive(t, live.ThumbnailAttemptedAt)
		assert.True(t, influencerPresetThumbnailProcessing(&live))
		// A competing worker that read the previously empty lease cannot claim
		// the row after this worker acquired it. This is a real affected-row check.
		competing := db.Model(&model.InfluencerPreset{}).Where("id = ? AND (thumbnail_attempted_at IS NULL OR thumbnail_attempted_at = ?)", preset.Id, 0).Updates(map[string]interface{}{"thumbnail_attempted_at": live.ThumbnailAttemptedAt + 1})
		require.NoError(t, competing.Error)
		assert.Zero(t, competing.RowsAffected)
		return &service.ImageThumbnail{Data: []byte("independent-webp-derivative"), Width: 2, Height: 3}, nil
	}
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	require.NoError(t, generateInfluencerPresetThumbnail(context.Background(), preset.Id))
	assert.Equal(t, 1, calls, "ready records must not be encoded twice")
	var stored model.InfluencerPreset
	require.NoError(t, db.First(&stored, preset.Id).Error)
	require.True(t, influencerPresetThumbnailReady(&stored))
	assert.Equal(t, "image/webp", stored.ThumbnailMime)
	assert.Equal(t, 2, stored.ThumbnailWidth)
	assert.Equal(t, 3, stored.ThumbnailHeight)
	assert.Equal(t, 1, stored.ThumbnailAttempts)
	assert.Zero(t, stored.ThumbnailAttemptedAt, "successful processing releases its persisted lease")
	assert.Equal(t, preset.PreviewKey, stored.PreviewKey)
	assert.Equal(t, preset.SheetKey, stored.SheetKey)
	assert.Equal(t, 940, stored.PreviewWidth)
	assert.Equal(t, 1672, stored.PreviewHeight)
	assert.Equal(t, original, driver.objects[stored.PreviewKey].body)
	assert.Equal(t, []byte("independent-webp-derivative"), driver.objects[stored.ThumbnailKey].body)
}
