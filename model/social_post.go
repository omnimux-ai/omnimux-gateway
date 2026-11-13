package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// SocialPost is a local ledger of a publishing request (Zernio post mirrored for query + isolation).
type SocialPost struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId           int       `json:"user_id" gorm:"not null;index"`
	Provider         string    `json:"provider" gorm:"type:varchar(32);not null;default:'zernio';index"`
	ProviderPostID   string    `json:"provider_post_id" gorm:"type:varchar(64);index"`
	IdempotencyKey   string    `json:"idempotency_key" gorm:"type:varchar(128);index:idx_social_post_idem,priority:2"`
	Status           string    `json:"status" gorm:"type:varchar(32);not null;index;default:'submitted'"`
	ContentPreview   string    `json:"content_preview" gorm:"type:varchar(512)"`
	RequestSnapshot  string    `json:"request_snapshot" gorm:"type:text"`
	ResultSnapshot   string    `json:"result_snapshot" gorm:"type:text"`
	QuotaCharged     int       `json:"quota_charged" gorm:"default:0"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (SocialPost) TableName() string {
	return "social_posts"
}

func CreateSocialPost(p *SocialPost) error {
	if p == nil || p.UserId <= 0 {
		return errors.New("invalid social post")
	}
	if p.Provider == "" {
		p.Provider = "zernio"
	}
	return DB.Create(p).Error
}

func GetSocialPostByIDForUser(id int64, userId int) (*SocialPost, error) {
	var p SocialPost
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetSocialPostByIdempotency(userId int, key string) (*SocialPost, error) {
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var p SocialPost
	err := DB.Where("user_id = ? AND idempotency_key = ?", userId, key).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetSocialPostByProviderPostID(provider, providerPostID string) (*SocialPost, error) {
	var p SocialPost
	err := DB.Where("provider = ? AND provider_post_id = ?", provider, providerPostID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func UpdateSocialPostResult(id int64, status, providerPostID, resultSnapshot string) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if providerPostID != "" {
		updates["provider_post_id"] = providerPostID
	}
	if resultSnapshot != "" {
		updates["result_snapshot"] = resultSnapshot
	}
	return DB.Model(&SocialPost{}).Where("id = ?", id).Updates(updates).Error
}
