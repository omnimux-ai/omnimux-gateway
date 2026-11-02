package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// SocialTenant maps an OmniMux user to an upstream social provider profile (Zernio).
type SocialTenant struct {
	ID                int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId            int       `json:"user_id" gorm:"not null;uniqueIndex:uk_social_tenant_user_provider,priority:1"`
	Provider          string    `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:uk_social_tenant_user_provider,priority:2;default:'zernio'"`
	ProviderProfileID string    `json:"provider_profile_id" gorm:"type:varchar(64);not null;index"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (SocialTenant) TableName() string {
	return "social_tenants"
}

func GetSocialTenant(userId int, provider string) (*SocialTenant, error) {
	var t SocialTenant
	err := DB.Where("user_id = ? AND provider = ?", userId, provider).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func CreateSocialTenant(t *SocialTenant) error {
	if t == nil || t.UserId <= 0 || t.Provider == "" || t.ProviderProfileID == "" {
		return errors.New("invalid social tenant")
	}
	return DB.Create(t).Error
}

func GetOrCreateSocialTenant(userId int, provider, profileID string) (*SocialTenant, error) {
	existing, err := GetSocialTenant(userId, provider)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	t := &SocialTenant{
		UserId:            userId,
		Provider:          provider,
		ProviderProfileID: profileID,
	}
	if err := CreateSocialTenant(t); err != nil {
		// race: another request created it
		if existing2, err2 := GetSocialTenant(userId, provider); err2 == nil {
			return existing2, nil
		}
		return nil, err
	}
	return t, nil
}
