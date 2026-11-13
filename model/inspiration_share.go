package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	InspirationShareStatusActive  = 1
	InspirationShareStatusExpired = 2
	InspirationShareStatusDeleted = 3

	DefaultInspirationExpireDuration = 72 * time.Hour

	BucketOmnimuxFiles       = "omnimux-files"
	BucketOmnimuxInspiration = "omnimux-inspiration"

	// InspirationShareSourceCloud marks a share whose row already lives in the
	// inspiration library. That library owns the data and serves it, so the
	// link does not depend on anything this gateway stores and is permanent by
	// construction.
	InspirationShareSourceCloud = "cloud"

	// InspirationShareSourceLocal marks data the caller brought along. It keeps
	// the 72-hour protection window unless an administrator asks for permanent
	// access.
	InspirationShareSourceLocal = "local"
)

// InspirationShare stores non-public, direct-link accessible inspiration records.
// The source decides the storage label and the default tier; the caller's
// identity decides whether a local upload may ask for permanent access.
type InspirationShare struct {
	Id            int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	ShareId       string     `json:"share_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserId        int        `json:"user_id" gorm:"index;not null"`
	IsAdmin       bool       `json:"is_admin" gorm:"index;default:false"`
	Source        string     `json:"source" gorm:"type:varchar(16);not null;default:'local';index"`
	Category      string     `json:"category" gorm:"type:varchar(128);not null"`
	Title         string     `json:"title" gorm:"type:varchar(255);not null"`
	Description   string     `json:"description" gorm:"type:text"`
	Prompt        string     `json:"prompt" gorm:"type:text;not null"`
	Model         string     `json:"model" gorm:"type:varchar(128)"`
	MediaType     string     `json:"media_type" gorm:"type:varchar(32);default:'video'"`
	MediaUrl      string     `json:"media_url" gorm:"type:text"`
	CoverUrl      string     `json:"cover_url" gorm:"type:text"`
	StorageBucket string     `json:"storage_bucket" gorm:"type:varchar(64);not null"`
	IsPublic      bool       `json:"is_public" gorm:"index;default:false"`
	CreatedAt     time.Time  `json:"created_at" gorm:"index;not null"`
	ExpiresAt     *time.Time `json:"expires_at" gorm:"index"`
	Status        int        `json:"status" gorm:"index;default:1"`

	// Permanent is a create-time instruction, not a stored column: the stored
	// authority is ExpiresAt == nil. A cloud share and an administrator's local
	// upload both ask for permanent access; an ordinary local upload does not.
	Permanent bool `json:"-" gorm:"-"`
}

func (InspirationShare) TableName() string {
	return "inspiration_shares"
}

// CreateInspirationShare creates a new inspiration share record with the storage
// label and expiry its source and request asked for.
func CreateInspirationShare(share *InspirationShare) error {
	if share == nil {
		return errors.New("share cannot be nil")
	}
	if strings.TrimSpace(share.Category) == "" {
		return errors.New("category is required")
	}
	if strings.TrimSpace(share.Title) == "" || strings.TrimSpace(share.Prompt) == "" {
		return errors.New("title and prompt data are required")
	}

	if share.ShareId == "" {
		share.ShareId = "insp_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
	}
	if share.CreatedAt.IsZero() {
		share.CreatedAt = time.Now()
	}
	if share.MediaType == "" {
		share.MediaType = "video"
	}
	if share.Source == "" {
		share.Source = InspirationShareSourceLocal
	}
	share.IsPublic = false // Non-public, accessible only via direct link
	share.Status = InspirationShareStatusActive

	if share.Permanent {
		share.ExpiresAt = nil // permanent access
		share.StorageBucket = BucketOmnimuxInspiration
	} else {
		share.StorageBucket = BucketOmnimuxFiles
		if share.ExpiresAt == nil {
			exp := share.CreatedAt.Add(DefaultInspirationExpireDuration)
			share.ExpiresAt = &exp
		}
	}

	return DB.Create(share).Error
}

// GetInspirationShareByShareId retrieves an active inspiration share by its share_id.
// If the record is expired, it returns ErrRecordNotFound or an expiration error.
func GetInspirationShareByShareId(shareId string) (*InspirationShare, error) {
	if shareId == "" {
		return nil, errors.New("empty share_id")
	}

	var share InspirationShare
	err := DB.Where("share_id = ? AND status = ?", shareId, InspirationShareStatusActive).First(&share).Error
	if err != nil {
		return nil, err
	}

	// Check if expired
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		// Mark as expired asynchronously or synchronously
		_ = DB.Model(&InspirationShare{}).Where("id = ?", share.Id).Update("status", InspirationShareStatusExpired)
		return nil, gorm.ErrRecordNotFound
	}

	return &share, nil
}
