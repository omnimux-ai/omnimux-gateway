package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fork: r2-file-storage-presign — pending ticket rows.
func setupFileRecordTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
	require.NoError(t, db.AutoMigrate(&File{}))
}

func TestUploadPendingFilesDoNotCountAgainstQuota(t *testing.T) {
	setupFileRecordTestDB(t)
	now := time.Now()

	require.NoError(t, CreateFile(&File{
		FileId: "file_pending", UserId: 7, FileName: "clip.mp4", FileSize: 4,
		MimeType: "video/mp4", StorageKey: "videos/file_pending_clip.mp4",
		FileUrl:   "https://cdn.example/videos/file_pending_clip.mp4",
		CreatedAt: now, ExpiresAt: now.Add(30 * time.Minute), Status: FileStatusUploadPending,
	}))

	count, err := CountActiveUserFiles(7)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "an issued ticket must not hold a quota slot")

	confirmed, err := GetFileByFileId("file_pending")
	require.NoError(t, err)
	assert.Equal(t, FileStatusUploadPending, confirmed.Status)

	expiresAt := now.Add(DefaultFileExpireDuration)
	require.NoError(t, ConfirmFileUpload(confirmed.Id, 9, "video/mp4", expiresAt))

	confirmed, err = GetFileByFileId("file_pending")
	require.NoError(t, err)
	assert.Equal(t, FileStatusActive, confirmed.Status)
	assert.Equal(t, int64(9), confirmed.FileSize)
	assert.WithinDuration(t, expiresAt, confirmed.ExpiresAt, time.Second)

	count, err = CountActiveUserFiles(7)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestExpiredSweepCoversAbandonedTickets(t *testing.T) {
	setupFileRecordTestDB(t)
	now := time.Now()

	require.NoError(t, CreateFile(&File{
		FileId: "file_abandoned", UserId: 7, FileName: "clip.mp4", FileSize: 4,
		MimeType: "video/mp4", StorageKey: "videos/file_abandoned_clip.mp4",
		FileUrl:   "https://cdn.example/videos/file_abandoned_clip.mp4",
		CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute), Status: FileStatusUploadPending,
	}))
	require.NoError(t, CreateFile(&File{
		FileId: "file_live_ticket", UserId: 7, FileName: "clip.mp4", FileSize: 4,
		MimeType: "video/mp4", StorageKey: "videos/file_live_ticket_clip.mp4",
		FileUrl:   "https://cdn.example/videos/file_live_ticket_clip.mp4",
		CreatedAt: now, ExpiresAt: now.Add(30 * time.Minute), Status: FileStatusUploadPending,
	}))
	require.NoError(t, CreateFile(&File{
		FileId: "file_active", UserId: 7, FileName: "clip.mp4", FileSize: 4,
		MimeType: "video/mp4", StorageKey: "videos/file_active_clip.mp4",
		FileUrl:   "https://cdn.example/videos/file_active_clip.mp4",
		CreatedAt: now, ExpiresAt: now.Add(DefaultFileExpireDuration), Status: FileStatusActive,
	}))

	expired, err := GetExpiredFiles(10)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, "file_abandoned", expired[0].FileId)
}
