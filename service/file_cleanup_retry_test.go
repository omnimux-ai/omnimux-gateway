package service

import (
	"context"
	"errors"
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

type retryFileCleanupDriver struct {
	storage.StorageDriver
	fail  bool
	calls int
}

func (driver *retryFileCleanupDriver) Delete(ctx context.Context, key string) error {
	driver.calls++
	if driver.fail {
		return errors.New("temporary storage failure")
	}
	return driver.StorageDriver.Delete(ctx, key)
}

func TestExpiredReferenceFileDeletionRetriesAfterStorageFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.File{}))
	previousDB, previousDriver := model.DB, storage.GetStorageDriver()
	model.DB = db
	driver := &retryFileCleanupDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir()), fail: true}
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() {
		model.DB = previousDB
		storage.SetGlobalDriver(previousDriver)
		sqlDB, e := db.DB()
		require.NoError(t, e)
		assert.NoError(t, sqlDB.Close())
	})
	record := &model.File{FileId: "frozen-retry", UserId: 41, FileName: "reference.mp4", FileSize: 3, StorageKey: "influencer-reference/frozen/reference.mp4", FileUrl: "https://files.example/reference.mp4", CreatedAt: time.Now().Add(-73 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour), Status: model.FileStatusActive}
	_, err = driver.Upload(context.Background(), record.StorageKey, []byte("raw"), "video/mp4")
	require.NoError(t, err)
	require.NoError(t, model.CreateFile(record))
	cleanupExpiredFiles()
	current, err := model.GetFileByFileId(record.FileId)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusActive, current.Status, "failed deletion must remain eligible for the next cleanup pass")
	assert.Equal(t, 1, driver.calls, "same failure is not retried in a tight loop")
	driver.fail = false
	cleanupExpiredFiles()
	current, err = model.GetFileByFileId(record.FileId)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusExpired, current.Status)
	assert.Equal(t, 2, driver.calls)
	_, _, _, err = driver.Get(context.Background(), record.StorageKey)
	assert.Error(t, err)
}
