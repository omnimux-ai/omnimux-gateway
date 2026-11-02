package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFileExpirationCleanup(t *testing.T) {
	if model.DB == nil {
		db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
		require.NoError(t, err)
		model.DB = db
	}
	require.NoError(t, model.DB.AutoMigrate(&model.File{}))

	tempDir := t.TempDir()
	driver := storage.NewLocalStorageDriver(tempDir)
	storage.SetGlobalDriver(driver)

	// Upload a file to storage
	ctx := context.Background()
	storageKey := "avatars/file_test_expired.png"
	_, err := driver.Upload(ctx, storageKey, []byte("fake png data"), "image/png")
	require.NoError(t, err)

	// Verify file exists in storage
	reader, _, _, err := driver.Get(ctx, storageKey)
	require.NoError(t, err)
	_ = reader.Close()

	// Insert file record into DB marked expired in the past
	expiredRecord := model.File{
		FileId:     "file_test_expired",
		UserId:     1,
		FileName:   "expired.png",
		FileSize:   13,
		MimeType:   "image/png",
		StorageKey: storageKey,
		FileUrl:    "https://example.com/expired.png",
		CreatedAt:  time.Now().Add(-80 * time.Hour),
		ExpiresAt:  time.Now().Add(-8 * time.Hour), // Expired 8 hours ago
		Status:     model.FileStatusActive,
	}
	require.NoError(t, model.CreateFile(&expiredRecord))

	// Run cleanup
	cleanupExpiredFiles()

	// Verify file record status changed to FileStatusExpired (2)
	updatedRecord, err := model.GetFileByFileId("file_test_expired")
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusExpired, updatedRecord.Status)

	// Verify file was deleted from storage
	_, _, _, err = driver.Get(ctx, storageKey)
	assert.Error(t, err)
}
