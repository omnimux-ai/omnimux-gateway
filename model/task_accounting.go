package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TaskConsumptionLogPending  = "pending"
	TaskConsumptionLogStarted  = "started"
	TaskConsumptionLogComplete = "complete"
	TaskConsumptionLogUnknown  = "unknown"
	TaskConsumptionLogDisabled = "disabled"
)

// CommitTaskConsumptionUsage commits existing native usage columns together
// with their task stage. It claims at most one separate consume-log attempt;
// it never changes wallet/token funding or manufactures a missing log.
func CommitTaskConsumptionUsage(task *Task, quota int) (bool, error) {
	if !task.HasVersionedSunburstImageInputs() || task.ID <= 0 || quota < 0 || quota > common.MaxQuota {
		return false, errors.New("invalid native task consumption")
	}
	var claimed bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", task.UserId).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var user User
		if err := lockForUpdate(tx).First(&user, task.UserId).Error; err != nil {
			return err
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		if current.UserId != task.UserId || current.TaskID != task.TaskID || !current.HasVersionedSunburstImageInputs() {
			return errors.New("native task accounting ownership lost")
		}
		task.PrivateData.GenerationReservationUnlogged = current.PrivateData.GenerationReservationUnlogged
		task.PrivateData.GenerationConsumptionLogState = current.PrivateData.GenerationConsumptionLogState
		if current.Status == TaskStatusFailure || !current.PrivateData.GenerationReservationUnlogged {
			return nil
		}
		if current.Status == TaskStatusWaiting || current.Status == TaskStatusClaimed || current.Status == TaskStatusDispatching || current.Quota != quota || current.ChannelId != task.ChannelId {
			return errors.New("native task accounting does not match its reservation")
		}
		if stage := current.PrivateData.GenerationConsumptionLogState; stage != "" && stage != TaskConsumptionLogPending {
			return errors.New("native task accounting stage is uncertain")
		}
		var channel Channel
		if err := lockForUpdate(tx).First(&channel, current.ChannelId).Error; err != nil {
			return err
		}
		if err := updateUserUsedQuotaAndRequestCount(current.UserId, quota, 1, tx); err != nil {
			return err
		}
		if err := updateChannelUsedQuota(current.ChannelId, quota, tx); err != nil {
			return err
		}
		current.PrivateData.GenerationReservationUnlogged = false
		current.PrivateData.GenerationConsumptionLogState = TaskConsumptionLogStarted
		result := tx.Model(&Task{}).Where("id = ? AND status = ?", current.ID, current.Status).Update("private_data", current.PrivateData)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("native task accounting commit lost")
		}
		task.PrivateData.GenerationReservationUnlogged = false
		task.PrivateData.GenerationConsumptionLogState = TaskConsumptionLogStarted
		claimed = true
		return nil
	})
	return claimed && err == nil, err
}

// FinishTaskConsumptionLog records only the outcome of the already-claimed
// native log attempt. An interrupted/unknown write is never blindly replayed.
func FinishTaskConsumptionLog(task *Task, state string) error {
	if !task.HasVersionedSunburstImageInputs() || (state != TaskConsumptionLogComplete && state != TaskConsumptionLogUnknown && state != TaskConsumptionLogDisabled) {
		return errors.New("invalid native task log state")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&Task{}).Where("id = ?", task.ID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		if current.UserId != task.UserId || current.TaskID != task.TaskID || current.PrivateData.GenerationReservationUnlogged || current.PrivateData.GenerationConsumptionLogState != TaskConsumptionLogStarted {
			return errors.New("native task log claim lost")
		}
		current.PrivateData.GenerationConsumptionLogState = state
		if err := tx.Model(&Task{}).Where("id = ?", current.ID).Update("private_data", current.PrivateData).Error; err != nil {
			return err
		}
		task.PrivateData.GenerationConsumptionLogState = state
		return nil
	})
}
