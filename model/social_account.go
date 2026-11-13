package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	SocialAccountStatusActive       = "active"
	SocialAccountStatusDisconnected = "disconnected"
	SocialAccountStatusUnknown      = "unknown"
)

// SocialAccount is a user-owned connected social account (provider-side id mapped locally).
type SocialAccount struct {
	ID                 int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId             int       `json:"user_id" gorm:"not null;index"`
	Provider           string    `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:uk_social_account_provider_ref,priority:1;default:'zernio'"`
	ProviderAccountID  string    `json:"provider_account_id" gorm:"type:varchar(64);not null;uniqueIndex:uk_social_account_provider_ref,priority:2"`
	ProviderProfileID  string    `json:"provider_profile_id" gorm:"type:varchar(64);index"`
	Platform           string    `json:"platform" gorm:"type:varchar(32);not null;index"`
	Username           string    `json:"username" gorm:"type:varchar(191)"`
	DisplayName        string    `json:"display_name" gorm:"type:varchar(191)"`
	Status             string    `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	SeatID             *int64    `json:"seat_id" gorm:"index"` // fork: social-seats 冗余；到期与空闲真相在 social_seats
	// fork: tiktok-direct — OAuth token vault (AES-GCM hex envelopes; never serialize plaintext).
	AccessTokenEncrypted  string     `json:"-" gorm:"type:text"`
	RefreshTokenEncrypted string     `json:"-" gorm:"type:text"`
	TokenExpiresAt        *time.Time `json:"token_expires_at,omitempty"`
	ScopesGranted         string     `json:"scopes_granted" gorm:"type:varchar(255)"`
	Metadata              string     `json:"metadata" gorm:"type:text"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (SocialAccount) TableName() string {
	return "social_accounts"
}

func ListSocialAccountsByUser(userId int, provider string) ([]*SocialAccount, error) {
	var rows []*SocialAccount
	q := DB.Where("user_id = ?", userId)
	if provider != "" {
		q = q.Where("provider = ?", provider)
	}
	err := q.Where("status = ?", SocialAccountStatusActive).Order("id desc").Find(&rows).Error
	return rows, err
}

func GetSocialAccountByIDForUser(id int64, userId int) (*SocialAccount, error) {
	var a SocialAccount
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func GetSocialAccountByProviderRef(provider, providerAccountID string) (*SocialAccount, error) {
	var a SocialAccount
	err := DB.Where("provider = ? AND provider_account_id = ?", provider, providerAccountID).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpsertSocialAccount inserts or updates by provider account id.
func UpsertSocialAccount(a *SocialAccount) error {
	if a == nil || a.UserId <= 0 || a.Provider == "" || a.ProviderAccountID == "" || a.Platform == "" {
		return errors.New("invalid social account")
	}
	if a.Status == "" {
		a.Status = SocialAccountStatusActive
	}
	existing, err := GetSocialAccountByProviderRef(a.Provider, a.ProviderAccountID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DB.Create(a).Error
		}
		return err
	}
	// Never reassign ownership across users.
	if existing.UserId != a.UserId {
		return errors.New("social account already bound to another user")
	}
	return DB.Model(existing).Updates(map[string]any{
		"provider_profile_id": a.ProviderProfileID,
		"platform":            a.Platform,
		"username":            a.Username,
		"display_name":        a.DisplayName,
		"status":              a.Status,
		"updated_at":          time.Now(),
	}).Error
}

func MarkSocialAccountDisconnected(id int64, userId int) error {
	res := DB.Model(&SocialAccount{}).
		Where("id = ? AND user_id = ?", id, userId).
		Updates(map[string]any{
			"status":     SocialAccountStatusDisconnected,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetOwnedSocialAccountsByIDs returns accounts for userId that match the given local IDs and are active.
func GetOwnedSocialAccountsByIDs(userId int, ids []int64) ([]*SocialAccount, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []*SocialAccount
	err := DB.Where("user_id = ? AND id IN ? AND status = ?", userId, ids, SocialAccountStatusActive).Find(&rows).Error
	return rows, err
}
