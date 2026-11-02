package model

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// Voice training reuses the native task row as the ownership record: the row's
// user_id is the owner and its task_id column holds the upstream voice code
// (speaker id), which the training adaptor writes as the upstream task id. That
// keeps voice ownership on an indexed column instead of scanning the JSON
// payload, and needs no model-specific table.

// GetVoiceCloneOwner resolves the owner of a trained voice.
//
// Returns found=false when the code is not a successfully trained voice of this
// platform — including every official preset voice, which has no task row and is
// therefore usable by anyone.
func GetVoiceCloneOwner(speakerID string) (userID int, found bool, err error) {
	code := strings.TrimSpace(speakerID)
	if code == "" {
		return 0, false, nil
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
//
// A code that is not a platform-trained voice returns false with owned=false:
// callers distinguish "this is somebody else's voice" from "this is not a
// platform voice at all" (official presets, which need no ownership check).
func UserOwnsVoiceClone(userID int, speakerID string) (owned bool, err error) {
	owner, found, err := GetVoiceCloneOwner(speakerID)
	if err != nil {
		return false, err
	}
	return found && owner == userID, nil
}

// CheckVoiceCloneAccess reports whether the user may synthesise with this voice.
//
// A code that is not a platform-trained voice — every official preset, plus any
// code the platform has no training record for — is allowed: ownership only
// constrains voices this platform minted for a user. A platform-trained voice is
// allowed only to its owner.
func CheckVoiceCloneAccess(userID int, speakerID string) (allowed bool, err error) {
	owner, found, err := GetVoiceCloneOwner(speakerID)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return owner == userID, nil
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
