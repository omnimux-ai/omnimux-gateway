package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func taskAccountingUserUsage(t *testing.T, userID int) int {
	t.Helper()
	used, _ := getUserUsageAccounting(t, userID)
	return used
}

func newVersionedTaskAccountingFixture(t *testing.T) (*model.Task, *relaycommon.RelayInfo, *gin.Context) {
	t.Helper()
	truncate(t)
	seedUser(t, 550, 10000)
	seedChannel(t, 550)
	task := makeTask(550, 550, 4000, 0, BillingSourceWallet, 0)
	task.Action = "influencer_sheet"
	task.Properties.OriginModelName = "gpt-image-2.5-sunburst"
	task.PrivateData.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text"}
	task.PrivateData.GenerationReservationUnlogged = true
	task.PrivateData.GenerationConsumptionLogState = model.TaskConsumptionLogPending
	require.NoError(t, model.DB.Create(task).Error)
	info := &relaycommon.RelayInfo{UserId: 550, OriginModelName: task.Properties.OriginModelName, UsingGroup: "default", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 550}, TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: task.Action}, PriceData: types.PriceData{ModelPrice: 0.01, Quota: 4000, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/images/generations", nil)
	c.Set(common.RequestIdKey, "native-versioned-accounting")
	return task, info, c
}

func TestVersionedTaskUsageTransactionRollsBackEveryPartialWrite(t *testing.T) {
	for _, table := range []string{"users", "channels", "tasks"} {
		t.Run(table, func(t *testing.T) {
			task, info, c := newVersionedTaskAccountingFixture(t)
			require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("test_accounting_failure", func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					if updates, ok := tx.Statement.Dest.(map[string]any); ok {
						if _, lockOnly := updates["status"]; lockOnly {
							return
						}
					}
					tx.AddError(errors.New("injected accounting transaction failure"))
				}
			}))
			LogTaskConsumption(c, info, task)
			require.NoError(t, model.DB.Callback().Update().Remove("test_accounting_failure"))
			var saved model.Task
			require.NoError(t, model.DB.First(&saved, task.ID).Error)
			assert.True(t, saved.PrivateData.GenerationReservationUnlogged)
			assert.Equal(t, model.TaskConsumptionLogPending, saved.PrivateData.GenerationConsumptionLogState)
			assert.Zero(t, taskAccountingUserUsage(t, task.UserId))
			assert.Zero(t, getChannelUsedQuota(t, task.ChannelId))
			assert.Zero(t, countLogs(t))
			LogTaskConsumption(c, info, task)
			LogTaskConsumption(c, info, task)
			assert.Equal(t, 4000, taskAccountingUserUsage(t, task.UserId))
			assert.EqualValues(t, 4000, getChannelUsedQuota(t, task.ChannelId))
			assert.Equal(t, int64(1), countLogs(t), "the known-rolled-back transaction can retry once without duplicating the native log")
		})
	}
}

func TestVersionedTaskAccountingCrashAfterUsageDoesNotReplayLogOrLoseRefundStage(t *testing.T) {
	task, info, c := newVersionedTaskAccountingFixture(t)
	var stale model.Task
	require.NoError(t, model.DB.First(&stale, task.ID).Error)
	claimed, err := model.CommitTaskConsumptionUsage(task, info.PriceData.Quota)
	require.NoError(t, err)
	require.True(t, claimed)
	// Process loss before the separate log write leaves started. A resumed
	// logger cannot know whether the write happened and must not replay it.
	LogTaskConsumption(c, info, task)
	assert.Zero(t, countLogs(t))
	assert.Equal(t, 4000, taskAccountingUserUsage(t, task.UserId))
	stale.Status = model.TaskStatusFailure
	won, err := stale.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	assert.False(t, stale.PrivateData.GenerationReservationUnlogged, "a poller loaded before accounting must merge the committed primary-DB stage")
	assert.Equal(t, model.TaskConsumptionLogStarted, stale.PrivateData.GenerationConsumptionLogState)
	finalizeTerminalTask(context.Background(), &scriptedPollingAdaptor{}, &stale, relaycommon.FailTaskInfo("confirmed failure"))
	assert.Zero(t, taskAccountingUserUsage(t, task.UserId))
	assert.Zero(t, getChannelUsedQuota(t, task.ChannelId))
	assert.Equal(t, 14000, getUserQuota(t, task.UserId))
	// A stale successful logger cannot resurrect a failed/refunded task.
	LogTaskConsumption(c, info, task)
	assert.Zero(t, taskAccountingUserUsage(t, task.UserId))
	assert.Equal(t, int64(1), countLogs(t), "only the native refund log exists")
	won, err = task.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	assert.False(t, won, "lost status CAS cannot authorize another terminal funding action")
}

func TestVersionedTaskLogFailureIsUnknownWithOneAttemptAndAtomicUsage(t *testing.T) {
	task, info, c := newVersionedTaskAccountingFixture(t)
	var stale model.Task
	require.NoError(t, model.DB.First(&stale, task.ID).Error)
	var writes int
	require.NoError(t, model.LOG_DB.Callback().Create().Before("gorm:create").Register("test_consume_log_failure", func(tx *gorm.DB) {
		if log, ok := tx.Statement.Dest.(*model.Log); ok && log.Type == model.LogTypeConsume {
			writes++
			tx.AddError(errors.New("injected separate log database failure"))
		}
	}))
	LogTaskConsumption(c, info, task)
	require.NoError(t, model.LOG_DB.Callback().Create().Remove("test_consume_log_failure"))
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	assert.False(t, saved.PrivateData.GenerationReservationUnlogged)
	assert.Equal(t, model.TaskConsumptionLogUnknown, saved.PrivateData.GenerationConsumptionLogState)
	assert.Equal(t, 4000, taskAccountingUserUsage(t, task.UserId))
	assert.EqualValues(t, 4000, getChannelUsedQuota(t, task.ChannelId))
	assert.Equal(t, 1, writes)
	LogTaskConsumption(c, info, task)
	assert.Zero(t, countLogs(t), "an unknown log outcome cannot be blindly replayed")
	assert.Equal(t, 4000, taskAccountingUserUsage(t, task.UserId))
	stale.Progress = "40%"
	won, err := stale.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	assert.Equal(t, model.TaskConsumptionLogUnknown, stale.PrivateData.GenerationConsumptionLogState)
	assert.False(t, stale.PrivateData.GenerationReservationUnlogged)
}

func TestVersionedTaskLogWrittenButOutcomeCommitLostNeverWritesItAgain(t *testing.T) {
	task, info, c := newVersionedTaskAccountingFixture(t)
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("test_log_outcome_failure", func(tx *gorm.DB) {
		if updates, ok := tx.Statement.Dest.(map[string]any); ok {
			if private, ok := updates["private_data"].(model.TaskPrivateData); ok && private.GenerationConsumptionLogState == model.TaskConsumptionLogComplete {
				tx.AddError(errors.New("injected final log-state write failure"))
			}
		}
	}))
	LogTaskConsumption(c, info, task)
	require.NoError(t, model.DB.Callback().Update().Remove("test_log_outcome_failure"))
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	assert.Equal(t, model.TaskConsumptionLogStarted, saved.PrivateData.GenerationConsumptionLogState)
	assert.False(t, saved.PrivateData.GenerationReservationUnlogged)
	assert.Equal(t, int64(1), countLogs(t))
	LogTaskConsumption(c, info, &saved)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, 4000, taskAccountingUserUsage(t, task.UserId))
	assert.EqualValues(t, 4000, getChannelUsedQuota(t, task.ChannelId))
}

func TestVersionedTaskDisabledConsumptionLogRecordsItsRealState(t *testing.T) {
	task, info, c := newVersionedTaskAccountingFixture(t)
	old := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = old })
	LogTaskConsumption(c, info, task)
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	assert.Equal(t, model.TaskConsumptionLogDisabled, saved.PrivateData.GenerationConsumptionLogState)
	assert.False(t, saved.PrivateData.GenerationReservationUnlogged)
	assert.Zero(t, countLogs(t))
	assert.Equal(t, 4000, taskAccountingUserUsage(t, task.UserId))
}

func TestVersionedTaskCompletedAccountingIsExactlyOnceAndRefundsRecordedUsage(t *testing.T) {
	task, info, c := newVersionedTaskAccountingFixture(t)
	LogTaskConsumption(c, info, task)
	LogTaskConsumption(c, info, task)
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	assert.Equal(t, model.TaskConsumptionLogComplete, saved.PrivateData.GenerationConsumptionLogState)
	assert.Equal(t, int64(1), countLogs(t))
	var user model.User
	require.NoError(t, model.DB.First(&user, task.UserId).Error)
	assert.Equal(t, 1, user.RequestCount)
	saved.Status = model.TaskStatusFailure
	won, err := saved.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	finalizeTerminalTask(context.Background(), &scriptedPollingAdaptor{}, &saved, relaycommon.FailTaskInfo("confirmed failure"))
	assert.Zero(t, taskAccountingUserUsage(t, task.UserId))
	assert.Zero(t, getChannelUsedQuota(t, task.ChannelId))
	assert.Equal(t, 14000, getUserQuota(t, task.UserId))
	assert.Equal(t, int64(2), countLogs(t))
}

func TestVersionedTaskFailureBeforeAccountingNeverAddsUsageAfterRefund(t *testing.T) {
	task, info, c := newVersionedTaskAccountingFixture(t)
	var failed model.Task
	require.NoError(t, model.DB.First(&failed, task.ID).Error)
	failed.Status = model.TaskStatusFailure
	won, err := failed.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	finalizeTerminalTask(context.Background(), &scriptedPollingAdaptor{}, &failed, relaycommon.FailTaskInfo("confirmed before accounting"))
	LogTaskConsumption(c, info, task)
	assert.Zero(t, taskAccountingUserUsage(t, task.UserId))
	assert.Zero(t, getChannelUsedQuota(t, task.ChannelId))
	assert.Equal(t, 14000, getUserQuota(t, task.UserId))
	assert.Equal(t, int64(1), countLogs(t), "late accounting cannot add a consume log after the native release")
}
