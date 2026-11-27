package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VoiceCloneNamespacePrefix marks platform-minted cloned voices whose ownership
// must be enforced fail-closed against the voice_clones registry.
const VoiceCloneNamespacePrefix = "omnimux_"

var ErrVoiceCloneOwnedByOther = errors.New("voice speaker_id belongs to another user")

// VoiceClone records durable ownership of a platform-cloned voice speaker ID.
type VoiceClone struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SpeakerID string `json:"speaker_id" gorm:"type:varchar(191);uniqueIndex;not null"`
	UserId    int    `json:"user_id" gorm:"index;not null"`
	TaskID    string `json:"task_id" gorm:"type:varchar(128);index;default:''"`
	Status    string `json:"status" gorm:"type:varchar(32);default:'active'"`
	CreatedAt int64  `json:"created_at" gorm:"type:bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"type:bigint"`
}

func (VoiceClone) TableName() string {
	return "voice_clones"
}

// BackfillVoiceClones populates voice_clones from historical VOICE_CLONE tasks.
// Succeeded tasks are backfilled before non-failed in-progress tasks, and failed
// tasks are skipped so an earlier failed attempt can never steal ownership from
// a later succeeded voice clone. Safe to run on every startup across SQLite,
// MySQL, and PostgreSQL.
func BackfillVoiceClones() error {
	if DB == nil {
		return nil
	}
	now := common.GetTimestamp()
	backfillQuery := func(query *gorm.DB, status string) error {
		var tasks []Task
		if err := query.Order("id ASC").Find(&tasks).Error; err != nil {
			return err
		}
		for i := range tasks {
			task := &tasks[i]
			if task.UserId <= 0 {
				continue
			}
			candidates := extractVoiceCloneSpeakerIDs(task)
			for _, speakerID := range candidates {
				createdAt := task.SubmitTime
				if createdAt <= 0 {
					createdAt = now
				}
				row := VoiceClone{
					SpeakerID: speakerID,
					UserId:    task.UserId,
					TaskID:    task.TaskID,
					Status:    status,
					CreatedAt: createdAt,
					UpdatedAt: now,
				}
				if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := backfillQuery(
		DB.Where("action = ? AND status = ?", constant.TaskActionVoiceClone, TaskStatusSuccess),
		"active",
	); err != nil {
		return err
	}
	return backfillQuery(
		DB.Where("action = ? AND status NOT IN (?, ?)", constant.TaskActionVoiceClone, TaskStatusSuccess, TaskStatusFailure),
		"pending",
	)
}

func extractVoiceCloneSpeakerIDs(task *Task) []string {
	seen := make(map[string]struct{}, 3)
	out := make([]string, 0, 3)
	add := func(raw string) {
		code := strings.TrimSpace(raw)
		if code == "" || code == "custom_speaker_id" {
			return
		}
		if _, ok := seen[code]; ok {
			return
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	if upstream := strings.TrimSpace(task.PrivateData.UpstreamTaskID); upstream != "" {
		add(upstream)
	}
	if strings.HasPrefix(strings.TrimSpace(task.TaskID), VoiceCloneNamespacePrefix) {
		add(task.TaskID)
	}
	if len(task.Data) > 0 {
		var data struct {
			SpeakerID       string `json:"speaker_id"`
			CustomSpeakerID string `json:"custom_speaker_id"`
		}
		if err := common.Unmarshal(task.Data, &data); err == nil {
			add(data.SpeakerID)
			add(data.CustomSpeakerID)
		}
	}
	return out
}

// ClaimOrVerifyVoiceCloneOwner atomically claims speakerID for userID before
// calling upstream voice training, or verifies that userID already owns it.
func ClaimOrVerifyVoiceCloneOwner(userID int, speakerID, taskID string) (claimedNew bool, err error) {
	if DB == nil {
		return false, nil
	}
	code := strings.TrimSpace(speakerID)
	if userID <= 0 || code == "" {
		return false, errors.New("invalid voice clone owner claim")
	}
	now := common.GetTimestamp()
	claim := VoiceClone{
		SpeakerID: code,
		UserId:    userID,
		TaskID:    strings.TrimSpace(taskID),
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	res := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
	if res.Error != nil {
		return false, res.Error
	}
	var existing VoiceClone
	if err := DB.Where("speaker_id = ?", code).First(&existing).Error; err != nil {
		return false, err
	}
	if existing.UserId != userID {
		return false, ErrVoiceCloneOwnedByOther
	}
	if claim.Id > 0 && existing.Id == claim.Id {
		return true, nil
	}
	if trimmedTaskID := strings.TrimSpace(taskID); trimmedTaskID != "" && existing.TaskID != trimmedTaskID {
		_ = DB.Model(&VoiceClone{}).Where("id = ?", existing.Id).Updates(map[string]any{
			"task_id":    trimmedTaskID,
			"updated_at": now,
		}).Error
	}
	return false, nil
}

// ReleaseVoiceCloneClaim deletes an uncommitted placeholder claim after an
// upstream submission error. Existing active claims are never removed.
func ReleaseVoiceCloneClaim(userID int, speakerID string) {
	if DB == nil || userID <= 0 {
		return
	}
	code := strings.TrimSpace(speakerID)
	if code == "" {
		return
	}
	_ = DB.Where("speaker_id = ? AND user_id = ? AND status = ?", code, userID, "pending").Delete(&VoiceClone{}).Error
}

// ConfirmVoiceCloneOwner marks speakerID as active for userID after upstream
// accepts the training request.
func ConfirmVoiceCloneOwner(userID int, speakerID, taskID string) error {
	if DB == nil || userID <= 0 {
		return nil
	}
	code := strings.TrimSpace(speakerID)
	if code == "" {
		return nil
	}
	now := common.GetTimestamp()
	row := VoiceClone{
		SpeakerID: code,
		UserId:    userID,
		TaskID:    strings.TrimSpace(taskID),
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	res := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}
	var existing VoiceClone
	if err := DB.Where("speaker_id = ?", code).First(&existing).Error; err != nil {
		return err
	}
	if existing.UserId != userID {
		return ErrVoiceCloneOwnedByOther
	}
	updates := map[string]any{
		"status":     "active",
		"updated_at": now,
	}
	if trimmedTaskID := strings.TrimSpace(taskID); trimmedTaskID != "" {
		updates["task_id"] = trimmedTaskID
	}
	return DB.Model(&VoiceClone{}).Where("id = ?", existing.Id).Updates(updates).Error
}

// GetVoiceCloneOwner resolves the owner of a trained voice from the dedicated
// voice_clones registry, with a read-only fallback to legacy task rows.
func GetVoiceCloneOwner(speakerID string) (userID int, found bool, err error) {
	code := strings.TrimSpace(speakerID)
	if code == "" || DB == nil {
		return 0, false, nil
	}
	var vc VoiceClone
	err = DB.Where("speaker_id = ?", code).First(&vc).Error
	if err == nil {
		return vc.UserId, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		if DB.Migrator().HasTable(&VoiceClone{}) {
			return 0, false, err
		}
	}

	var task Task
	err = DB.
		Where("task_id = ? AND action = ? AND status = ?", code, constant.TaskActionVoiceClone, TaskStatusSuccess).
		First(&task).Error
	exist, err := RecordExist(err)
	if err != nil {
		return 0, false, err
	}
	if !exist {
		return 0, false, nil
	}
	return task.UserId, true, nil
}

// UserOwnsVoiceClone reports whether the given voice code belongs to the user.
func UserOwnsVoiceClone(userID int, speakerID string) (owned bool, err error) {
	owner, found, err := GetVoiceCloneOwner(speakerID)
	if err != nil {
		return false, err
	}
	return found && userID > 0 && owner == userID, nil
}

// CheckVoiceCloneAccess reports whether the user may synthesise with this voice.
//
// Platform-namespaced voices (omnimux_*) fail closed: if no ownership record
// exists or the owner does not match userID, synthesis is denied. Official
// preset voices outside the platform namespace remain usable by everyone.
func CheckVoiceCloneAccess(userID int, speakerID string) (allowed bool, err error) {
	code := strings.TrimSpace(speakerID)
	if code == "" {
		return false, nil
	}
	owner, found, err := GetVoiceCloneOwner(code)
	if err != nil {
		return false, err
	}
	if !found {
		if strings.HasPrefix(code, VoiceCloneNamespacePrefix) {
			return false, nil
		}
		return true, nil
	}
	return userID > 0 && owner == userID, nil
}

// GetVoiceCloneTasksByUser lists a user's successfully trained voices, newest
// first, so a caller can render the user's own voice library.
func GetVoiceCloneTasksByUser(userID int) ([]*Task, error) {
	if userID <= 0 {
		return nil, nil
	}
	var tasks []*Task
	err := DB.
		Where("user_id = ? AND action = ? AND status = ?", userID, constant.TaskActionVoiceClone, TaskStatusSuccess).
		Order("id DESC").
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}
