package controller

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"testing"
)

// Advances an accepted receipt through the actual worker (local providers only).
func dispatchGenerationQueueTest(t *testing.T, userID int, body []byte) *model.Task {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	oldMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	common.IsMasterNode = oldMaster
	require.NoError(t, i18n.Init())
	var receipt struct {
		TaskID string `json:"task_id"`
	}
	require.NoError(t, common.Unmarshal(body, &receipt))
	require.NotEmpty(t, receipt.TaskID)
	scanner := &model.SystemTask{TaskID: model.GenerateTaskID()}
	require.NoError(t, model.DB.Where("type = ?", model.SystemTaskTypeGenerationQueue).Delete(&model.SystemTaskLock{}).Error)
	require.NoError(t, model.DB.Create(&model.SystemTaskLock{Type: model.SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "test", LockedUntil: common.GetTimestamp() + 60}).Error)
	tasks, err := model.ClaimGenerationTasks(context.Background(), userID, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	dispatchInfluencerGeneration(context.Background(), tasks[0], scanner, "test")
	stored, exists, err := model.GetByTaskId(userID, receipt.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	return stored
}
