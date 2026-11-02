package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const influencerRequestRecoverySeconds int64 = 7 * 24 * 60 * 60

var (
	ErrInfluencerRequestConflict     = errors.New("creation request content conflicts with the accepted request")
	ErrInfluencerRequestExpired      = errors.New("creation request recovery has expired")
	ErrInfluencerRequestUnauthorized = errors.New("creation request authorization is no longer valid")
	ErrInfluencerRequestInvalid      = errors.New("creation request is invalid")
	influencerRequestIDPattern       = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
	influencerPayloadHashPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// InfluencerGenerationRequest is a minimal durable acceptance receipt. Native
// Task rows remain the only generation queue and billing lifecycle.
type InfluencerGenerationRequest struct {
	UserID int `json:"-" gorm:"primaryKey;autoIncrement:false"`
	// A digest gives request IDs byte-exact identity on every supported DB,
	// including MySQL installations with case-insensitive default collations.
	RequestKey        string `json:"-" gorm:"type:varchar(64);primaryKey"`
	RequestID         string `json:"request_id" gorm:"type:varchar(128);not null"`
	PayloadHash       string `json:"-" gorm:"type:varchar(64);not null"`
	TaskID            string `json:"task_id" gorm:"type:varchar(191);not null;index"`
	CreatedAt         int64  `json:"created_at" gorm:"not null"`
	RecoveryExpiresAt int64  `json:"recovery_expires_at" gorm:"not null"`
}

// CommitInfluencerGenerationRequest accepts a WAITING native Task and its
// receipt atomically. Browser callers pass zero only after validating their
// browser identity; PAT callers must supply the current access-token row ID.
// Exactly identical accepted requests return the original task without enqueue.
func CommitInfluencerGenerationRequest(ctx context.Context, task *Task, requestID, payloadHash string, accessTokenID int, requiredScope string) (*Task, bool, error) {
	if task == nil || task.UserId <= 0 || task.ID != 0 || task.TaskID == "" || task.Status != TaskStatusWaiting || task.PrivateData.GenerationQueue == nil || (task.Action != "influencer_sheet" && task.Action != "influencer_motion") || !influencerRequestIDPattern.MatchString(requestID) || !influencerPayloadHashPattern.MatchString(payloadHash) {
		return nil, false, ErrInfluencerRequestInvalid
	}
	if accessTokenID < 0 || requiredScope != "influencer_creation:write" {
		return nil, false, ErrInfluencerRequestUnauthorized
	}
	var accepted *Task
	var replay bool
	var recoveryErr error
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", task.UserId).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var owner User
		if err := lockForUpdate(tx).Select("id", "status").First(&owner, task.UserId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInfluencerRequestUnauthorized
			}
			return err
		}
		if owner.Status != common.UserStatusEnabled {
			return ErrInfluencerRequestUnauthorized
		}
		if accessTokenID > 0 {
			var token UserAccessToken
			if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", accessTokenID, owner.Id).First(&token).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrInfluencerRequestUnauthorized
				}
				return err
			}
			if token.Expired(time.Now().Unix()) {
				return ErrInfluencerRequestUnauthorized
			}
			explicit := false
			for _, scope := range token.GetScopes() {
				if scope == requiredScope {
					explicit = true
					break
				}
			}
			if !explicit {
				return ErrInfluencerRequestUnauthorized
			}
		}
		key := sha256.Sum256([]byte(requestID))
		receipt := InfluencerGenerationRequest{UserID: owner.Id, RequestKey: hex.EncodeToString(key[:])}
		err := lockForUpdate(tx).Where("user_id = ? AND request_key = ?", receipt.UserID, receipt.RequestKey).First(&receipt).Error
		if err == nil {
			if receipt.RequestID != requestID || receipt.PayloadHash != payloadHash {
				return ErrInfluencerRequestConflict
			}
			accepted, recoveryErr = recoverInfluencerRequestTask(tx, &receipt, time.Now().Unix())
			if recoveryErr != nil && !errors.Is(recoveryErr, ErrInfluencerRequestExpired) {
				return recoveryErr
			}
			replay = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Do not mutate the caller's ID until both inserts have committed.
		candidate := *task
		if err := tx.Create(&candidate).Error; err != nil {
			return err
		}
		receipt.RequestID = requestID
		receipt.PayloadHash = payloadHash
		receipt.TaskID = candidate.TaskID
		receipt.CreatedAt = time.Now().Unix()
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		accepted = &candidate
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if recoveryErr != nil {
		return nil, replay, recoveryErr
	}
	if !replay {
		*task = *accepted
	}
	return accepted, replay, nil
}

// GetInfluencerGenerationRequest returns only an owner's acceptance receipt.
// Missing acceptance is gorm.ErrRecordNotFound. Expiry or an unavailable native
// task returns the retained receipt alongside ErrInfluencerRequestExpired.
func GetInfluencerGenerationRequest(ctx context.Context, userID int, requestID string) (*InfluencerGenerationRequest, error) {
	if userID <= 0 || !influencerRequestIDPattern.MatchString(requestID) {
		return nil, gorm.ErrRecordNotFound
	}
	key := sha256.Sum256([]byte(requestID))
	var receipt InfluencerGenerationRequest
	var recoveryErr error
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire SQLite's write lock before any reads, so setting the first
		// terminal expiry cannot require an unsafe read-to-write upgrade.
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		if err := lockForUpdate(tx).Where("user_id = ? AND request_key = ?", userID, hex.EncodeToString(key[:])).First(&receipt).Error; err != nil {
			return err
		}
		if receipt.RequestID != requestID {
			return gorm.ErrRecordNotFound
		}
		_, recoveryErr = recoverInfluencerRequestTask(tx, &receipt, time.Now().Unix())
		if errors.Is(recoveryErr, ErrInfluencerRequestExpired) {
			return nil // commit the permanent recovery fence even on expiry
		}
		return recoveryErr
	})
	if err != nil {
		return nil, err
	}
	return &receipt, recoveryErr
}

func recoverInfluencerRequestTask(tx *gorm.DB, receipt *InfluencerGenerationRequest, now int64) (*Task, error) {
	if receipt.RecoveryExpiresAt > 0 && receipt.RecoveryExpiresAt <= now {
		return nil, ErrInfluencerRequestExpired
	}
	var task Task
	err := lockForUpdate(tx).Where("user_id = ? AND task_id = ? AND action IN ?", receipt.UserID, receipt.TaskID, generationActions).First(&task).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || !task.ResultRetrievable() {
		if receipt.RecoveryExpiresAt == 0 || receipt.RecoveryExpiresAt > now {
			receipt.RecoveryExpiresAt = now
			if err := tx.Model(receipt).Update("recovery_expires_at", now).Error; err != nil {
				return nil, err
			}
		}
		return nil, ErrInfluencerRequestExpired
	}
	if task.Status != TaskStatusSuccess && task.Status != TaskStatusFailure {
		// Recovery for an active native task is not bounded by creation time.
		receipt.RecoveryExpiresAt = 0
		return &task, nil
	}
	if receipt.RecoveryExpiresAt == 0 {
		completed := task.FinishTime
		if completed <= 0 {
			completed = task.UpdatedAt
		}
		if completed <= 0 || completed > now {
			completed = now
		}
		receipt.RecoveryExpiresAt = completed + influencerRequestRecoverySeconds
		if err := tx.Model(receipt).Update("recovery_expires_at", receipt.RecoveryExpiresAt).Error; err != nil {
			return nil, err
		}
	}
	if receipt.RecoveryExpiresAt <= now {
		return nil, ErrInfluencerRequestExpired
	}
	return &task, nil
}
