package model

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	GenerationObjectSwapRunning = "running"
	GenerationObjectSwapReady   = "ready"
	GenerationObjectSwapFailed  = "failed"
	GenerationObjectSwapUnknown = "unknown"
)

// GenerationObjectSwapPreparation records the joint-media helper before video
// submission. Details contains credential-free audit data and optional usage;
// missing usage remains unknown, rather than being recorded as zero.
type GenerationObjectSwapPreparation struct {
	State      string          `json:"state"`
	StartedAt  int64           `json:"started_at"`
	FinishedAt int64           `json:"finished_at,omitempty"`
	Prompt     string          `json:"prompt,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

// BeginGenerationObjectSwapPreparation durably fences one helper call. An
// existing running record has no confirmed result, so it becomes unknown and
// is never restarted, including after a scanner recovers the task's claim.
func BeginGenerationObjectSwapPreparation(ctx context.Context, task *Task, scanner *SystemTask, runnerID string) (*GenerationObjectSwapPreparation, bool, error) {
	var saved TaskPrivateData
	var begun bool
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockGenerationObjectSwapTask(tx, task, scanner, runnerID)
		if err != nil {
			return err
		}
		preparation := current.PrivateData.ObjectSwapPreparation
		if preparation == nil {
			current.PrivateData.ObjectSwapPreparation = &GenerationObjectSwapPreparation{State: GenerationObjectSwapRunning, StartedAt: common.GetTimestamp()}
			begun = true
		} else {
			switch preparation.State {
			case GenerationObjectSwapRunning:
				preparation.State = GenerationObjectSwapUnknown
				preparation.FinishedAt = common.GetTimestamp()
			case GenerationObjectSwapReady, GenerationObjectSwapFailed, GenerationObjectSwapUnknown:
				saved = current.PrivateData
				return nil
			default:
				return errors.New("invalid object swap preparation state")
			}
		}
		if err := tx.Model(&Task{}).Where("id = ? AND status = ?", current.ID, TaskStatusClaimed).Update("private_data", current.PrivateData).Error; err != nil {
			return err
		}
		saved = current.PrivateData
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	task.PrivateData = saved
	return saved.ObjectSwapPreparation, begun, nil
}

// FinishGenerationObjectSwapPreparation accepts only the current claim's
// running preparation. A stale or unconfirmed caller cannot replace evidence
// saved by recovery or enable a second paid call.
func FinishGenerationObjectSwapPreparation(ctx context.Context, task *Task, scanner *SystemTask, runnerID string, result GenerationObjectSwapPreparation) error {
	if result.State != GenerationObjectSwapReady && result.State != GenerationObjectSwapFailed && result.State != GenerationObjectSwapUnknown {
		return errors.New("invalid object swap preparation result")
	}
	if result.State == GenerationObjectSwapReady && (strings.TrimSpace(result.Prompt) == "" || len(result.Details) == 0) {
		return errors.New("invalid object swap preparation prompt")
	}
	if result.State != GenerationObjectSwapReady && result.Prompt != "" {
		return errors.New("invalid object swap preparation prompt")
	}
	if len(result.Details) > 0 {
		var details map[string]any
		if err := common.Unmarshal(result.Details, &details); err != nil || details == nil {
			return errors.New("invalid object swap preparation details")
		}
	}
	var saved TaskPrivateData
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockGenerationObjectSwapTask(tx, task, scanner, runnerID)
		if err != nil {
			return err
		}
		preparation := current.PrivateData.ObjectSwapPreparation
		if preparation == nil || preparation.State != GenerationObjectSwapRunning {
			return errors.New("object swap preparation is no longer running")
		}
		result.StartedAt = preparation.StartedAt
		result.FinishedAt = common.GetTimestamp()
		result.Details = append(json.RawMessage(nil), result.Details...)
		current.PrivateData.ObjectSwapPreparation = &result
		if err := tx.Model(&Task{}).Where("id = ? AND status = ?", current.ID, TaskStatusClaimed).Update("private_data", current.PrivateData).Error; err != nil {
			return err
		}
		saved = current.PrivateData
		return nil
	})
	if err == nil {
		task.PrivateData = saved
	}
	return err
}

// lockGenerationObjectSwapTask follows the native queue lock order, fencing
// both the global scanner lease and the account's current task claim.
func lockGenerationObjectSwapTask(tx *gorm.DB, task *Task, scanner *SystemTask, runnerID string) (*Task, error) {
	if task == nil || scanner == nil || task.ID <= 0 || task.UserId <= 0 || task.TaskID == "" || scanner.TaskID == "" || runnerID == "" || task.PrivateData.GenerationQueue == nil {
		return nil, errors.New("invalid object swap preparation ownership")
	}
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		if err := tx.Model(&User{}).Where("id = ?", task.UserId).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
			return nil, err
		}
	}
	if err := lockGenerationScannerLease(tx, scanner, runnerID); err != nil {
		return nil, err
	}
	var user User
	if err := lockForUpdate(tx).First(&user, task.UserId).Error; err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New("generation account is disabled")
	}
	var current Task
	if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
		return nil, err
	}
	queue := current.PrivateData.GenerationQueue
	if current.UserId != task.UserId || current.TaskID != task.TaskID || current.Action != "influencer_motion" || current.Status != TaskStatusClaimed || queue == nil || queue.ClaimOwner != task.PrivateData.GenerationQueue.ClaimOwner || queue.ClaimOwner != scanner.TaskID || queue.SendStartedAt != 0 {
		return nil, errors.New("object swap preparation ownership lost")
	}
	return &current, nil
}
