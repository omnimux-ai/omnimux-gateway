package model

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	FileStatusActive  = 1
	FileStatusExpired = 2
	FileStatusDeleted = 3
	// FileStatusUploadPending marks a row whose upload ticket was issued but
	// whose object has not been verified yet. Pending rows are deliberately not
	// counted as active, so an abandoned ticket never consumes user quota.
	FileStatusUploadPending = 4
	// Rejected tickets stay tracked until their write window has elapsed.
	FileStatusUploadRejected = 5

	DefaultUserMaxFiles       = 2000
	DefaultFileExpireDuration = 72 * time.Hour
)

// File stores uploaded file records, expiration time, and storage locations.
type File struct {
	Id           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	FileId       string    `json:"file_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserId       int       `json:"user_id" gorm:"index;not null"`
	FileName     string    `json:"file_name" gorm:"type:varchar(255);not null"`
	OriginalName string    `json:"original_name" gorm:"type:varchar(255)"`
	FileSize     int64     `json:"file_size" gorm:"not null"`
	MimeType     string    `json:"mime_type" gorm:"type:varchar(128);not null"`
	StorageKey   string    `json:"storage_key" gorm:"type:varchar(512);not null"`
	UploadPath   string    `json:"upload_path" gorm:"type:varchar(255)"`
	FileUrl      string    `json:"file_url" gorm:"type:text;not null"`
	DownloadUrl  string    `json:"download_url" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at" gorm:"index;not null"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"index;not null"`
	Status       int       `json:"status" gorm:"index;default:1"`
	// VideoMetadata carries the browser declaration for an
	// influencer_reference_video upload only; every other file keeps NULL.
	// The json serializer writes TEXT as a string on all three dialects, which
	// keeps PostgreSQL from encoding []byte as bytea.
	VideoMetadata json.RawMessage `json:"-" gorm:"type:text;serializer:json"`
}

func (File) TableName() string {
	return "files"
}

// CreateFile inserts a new file record into the database.
func CreateFile(file *File) error {
	if file == nil {
		return errors.New("file cannot be nil")
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now()
	}
	if file.ExpiresAt.IsZero() {
		file.ExpiresAt = file.CreatedAt.Add(DefaultFileExpireDuration)
	}
	if file.Status == 0 {
		file.Status = FileStatusActive
	}
	return DB.Create(file).Error
}

// GetFileByFileId finds a single file by public file_id.
func GetFileByFileId(fileId string) (*File, error) {
	var f File
	err := DB.Where("file_id = ?", fileId).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetUniqueFileByURL restores a frozen reference without accepting ambiguous records.
func GetUniqueFileByURL(ctx context.Context, fileURL string) (*File, error) {
	var files []File
	if err := DB.WithContext(ctx).Where("file_url = ?", fileURL).Limit(2).Find(&files).Error; err != nil {
		return nil, err
	}
	if len(files) != 1 || files[0].FileUrl != fileURL {
		return nil, errors.New("file URL is missing or ambiguous")
	}
	return &files[0], nil
}

// CountActiveUserFiles returns the count of active, non-expired files for a user.
func CountActiveUserFiles(userId int) (int64, error) {
	var count int64
	now := time.Now()
	err := DB.Model(&File{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userId, FileStatusActive, now).
		Count(&count).Error
	return count, err
}

// GetExpiredFiles retrieves a batch of files that have exceeded expires_at:
// active rows whose 72-hour window closed, plus upload-pending rows whose ticket
// window closed without ever being confirmed. A pending row may have no object
// at all; deletion tolerates a missing key.
func GetExpiredFiles(batchSize int) ([]*File, error) {
	var files []*File
	now := time.Now()
	err := DB.Where("status IN ? AND expires_at <= ?", []int{FileStatusActive, FileStatusUploadPending, FileStatusUploadRejected}, now).
		Limit(batchSize).
		Find(&files).Error
	return files, err
}

// ConfirmFileUpload promotes an upload-pending record to a normal active file
// and records the size and content type observed in storage.
func ConfirmFileUpload(id int64, fileSize int64, mimeType string, expiresAt time.Time) error {
	result := DB.Model(&File{}).Where("id = ? AND status = ? AND expires_at > ?", id, FileStatusUploadPending, time.Now()).Updates(map[string]any{
		"status": FileStatusActive, "file_size": fileSize, "mime_type": mimeType, "expires_at": expiresAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("upload is no longer pending")
	}
	return nil
}

// CreateActiveFileWithinQuota shares the owner lock with direct confirmations.
func CreateActiveFileWithinQuota(file *File) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var owner User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", file.UserId).First(&owner).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&File{}).Where("user_id = ? AND status = ? AND expires_at > ?", file.UserId, FileStatusActive, time.Now()).Count(&count).Error; err != nil {
			return err
		}
		if count >= DefaultUserMaxFiles {
			return errors.New("文件配额已耗尽")
		}
		return tx.Create(file).Error
	})
}

// ConfirmDirectFileUpload serializes the quota check and promotion for one owner.
func ConfirmDirectFileUpload(id int64, userID int, size int64, contentType string, expiresAt time.Time) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var owner User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userID).First(&owner).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&File{}).Where("user_id = ? AND status = ? AND expires_at > ?", userID, FileStatusActive, time.Now()).Count(&count).Error; err != nil {
			return err
		}
		if count >= DefaultUserMaxFiles {
			return errors.New("文件配额已耗尽")
		}
		result := tx.Model(&File{}).Where("id = ? AND user_id = ? AND status = ? AND expires_at > ?", id, userID, FileStatusUploadPending, time.Now()).Updates(map[string]any{
			"status": FileStatusActive, "file_size": size, "mime_type": contentType, "expires_at": expiresAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("upload is no longer pending")
		}
		return nil
	})
}

// RejectPendingFileUpload cannot invalidate an already confirmed object.
func RejectPendingFileUpload(id int64) (bool, error) {
	result := DB.Model(&File{}).Where("id = ? AND status = ?", id, FileStatusUploadPending).Update("status", FileStatusUploadRejected)
	return result.RowsAffected == 1, result.Error
}

// MarkFileStatus updates the status of a file record.
func MarkFileStatus(id int64, status int) error {
	return DB.Model(&File{}).Where("id = ?", id).Update("status", status).Error
}

// DeleteFilePermanently removes a file record from the database.
func DeleteFilePermanently(id int64) error {
	return DB.Where("id = ?", id).Delete(&File{}).Error
}
