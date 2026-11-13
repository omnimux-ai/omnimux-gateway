package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileQuotaIsSharedByDirectAndServerUploads(t *testing.T) {
	setupFileRecordTestDB(t)
	require.NoError(t, DB.AutoMigrate(&User{}))
	require.NoError(t, DB.Create(&User{Id: 7, Username: "quota-owner", Status: 1}).Error)
	for i := 0; i < DefaultUserMaxFiles-1; i++ {
		require.NoError(t, CreateFile(&File{FileId: fmt.Sprintf("held-%d", i), UserId: 7, FileName: "a", MimeType: "image/png", StorageKey: "a", FileUrl: "https://files.example/a", Status: FileStatusActive}))
	}
	pending := &File{FileId: "direct", UserId: 7, FileName: "a", FileSize: 4, MimeType: "image/png", StorageKey: "direct", FileUrl: "https://files.example/direct", Status: FileStatusUploadPending}
	require.NoError(t, CreateFile(pending))
	require.NoError(t, ConfirmDirectFileUpload(pending.Id, 7, 4, "image/png", time.Now().Add(DefaultFileExpireDuration)))
	server := &File{FileId: "server", UserId: 7, FileName: "a", FileSize: 4, MimeType: "image/png", StorageKey: "server", FileUrl: "https://files.example/server", Status: FileStatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(DefaultFileExpireDuration)}
	assert.Error(t, CreateActiveFileWithinQuota(server))
	count, err := CountActiveUserFiles(7)
	require.NoError(t, err)
	assert.Equal(t, int64(DefaultUserMaxFiles), count)
}

func TestConfirmFileUploadDoesNotReactivateCompletedOrExpiredRow(t *testing.T) {
	setupFileRecordTestDB(t)
	for _, status := range []int{FileStatusActive, FileStatusExpired, FileStatusDeleted} {
		record := &File{FileId: time.Now().Format("150405.000000000"), UserId: 7, FileName: "a", FileSize: 4, MimeType: "image/png", StorageKey: "a", FileUrl: "https://files.example/a", ExpiresAt: time.Now().Add(time.Hour), Status: status}
		require.NoError(t, CreateFile(record))
		err := ConfirmFileUpload(record.Id, 99, "text/html", time.Now().Add(72*time.Hour))
		assert.Error(t, err)
		after, e := GetFileByFileId(record.FileId)
		require.NoError(t, e)
		assert.Equal(t, status, after.Status)
		assert.Equal(t, int64(4), after.FileSize)
	}
}
