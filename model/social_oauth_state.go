package model

// fork: tiktok-direct — one-time OAuth state for TikTok direct account binding.
// Rows are short-lived (default 10 minutes), single-use, deleted on consume.

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const SocialOAuthStateTTL = 10 * time.Minute

// SocialOAuthState binds an OAuth `state` to the OmniMux user who started Connect.
type SocialOAuthState struct {
	State        string    `json:"state" gorm:"type:varchar(128);primaryKey"`
	UserId       int       `json:"user_id" gorm:"not null;index"`
	Provider     string    `json:"provider" gorm:"type:varchar(32);not null;index"`
	RedirectAfter string   `json:"redirect_after" gorm:"type:varchar(512)"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt    time.Time `json:"created_at"`
}

func (SocialOAuthState) TableName() string {
	return "social_oauth_states"
}

func CreateSocialOAuthState(s *SocialOAuthState) error {
	if s == nil || s.State == "" || s.UserId <= 0 || s.Provider == "" {
		return errors.New("invalid social oauth state")
	}
	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = time.Now().UTC().Add(SocialOAuthStateTTL)
	}
	return DB.Create(s).Error
}

// ConsumeSocialOAuthState atomically loads and deletes the state (single use).
func ConsumeSocialOAuthState(state, provider string) (*SocialOAuthState, error) {
	if state == "" || provider == "" {
		return nil, errors.New("invalid social oauth state")
	}
	var s SocialOAuthState
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("state = ? AND provider = ?", state, provider).First(&s).Error; err != nil {
			return err
		}
		res := tx.Where("state = ? AND provider = ?", state, provider).Delete(&SocialOAuthState{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		return nil, errors.New("social oauth state expired")
	}
	return &s, nil
}
