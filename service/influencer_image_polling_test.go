package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The fetch adaptor supplies local response bytes only. Every transition and
// refund below uses the native persisted task and wallet primitives.
func TestSunburstAcceptedPollingRetainsUnknownThroughRestartAndTimeout(t *testing.T) {
	for _, test := range []struct {
		name    string
		adaptor scriptedPollingAdaptor
	}{
		{"unverified success envelope", scriptedPollingAdaptor{body: []byte(`{"code":"success","data":{"status":"SUCCESS"}}`), parseErr: errors.New("missing image")}},
		{"empty parsed status cannot fall back", scriptedPollingAdaptor{body: []byte(`{"code":"success","data":{"status":"SUCCESS"}}`), parse: &relaycommon.TaskInfo{}}},
		{"original task temporarily missing", scriptedPollingAdaptor{statusCode: http.StatusNotFound}},
		{"supplier temporarily unavailable", scriptedPollingAdaptor{statusCode: http.StatusServiceUnavailable}},
		{"transport result unknown", scriptedPollingAdaptor{fetchErr: errors.New("connection interrupted")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			truncate(t)
			const userID, tokenID, channelID = 540, 540, 540
			const initialQuota, preConsumed, tokenRemain = 10000, 4000, 7000
			seedUser(t, userID, initialQuota)
			seedToken(t, tokenID, userID, "sk-sunburst-poll", tokenRemain)
			ch := &model.Channel{Id: channelID, Type: constant.ChannelTypeOpenAI, Name: "image-poll", Status: common.ChannelStatusEnabled}
			task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
			task.TaskID = "task_sunburst_original"
			task.Action = "influencer_sheet"
			task.SubmitTime = time.Now().Add(-2 * time.Hour).Unix()
			task.Properties.OriginModelName = "gpt-image-2.5-sunburst"
			task.PrivateData.UpstreamTaskID = "accepted-original-upstream"
			task.PrivateData.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text"}
			require.NoError(t, model.DB.Create(task).Error)
			oldMax, oldTimeout := constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes
			constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes = 1, 1
			t.Cleanup(func() { constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes = oldMax, oldTimeout })
			sweepTimedOutTasks(context.Background())
			var initiallyTimedOut model.Task
			require.NoError(t, model.DB.First(&initiallyTimedOut, task.ID).Error)
			assert.EqualValues(t, model.TaskStatusInProgress, initiallyTimedOut.Status)
			assert.True(t, initiallyTimedOut.PrivateData.ImageResultUnverified)
			assert.Equal(t, preConsumed, initiallyTimedOut.Quota)

			for round := 0; round < 2; round++ {
				// Reload each round so the protection is established by database
				// state, rather than an in-process object surviving a restart.
				var persisted model.Task
				require.NoError(t, model.DB.First(&persisted, task.ID).Error)
				require.NoError(t, updateVideoSingleTask(context.Background(), &test.adaptor, ch, persisted.GetUpstreamTaskID(), map[string]*model.Task{persisted.GetUpstreamTaskID(): &persisted}))
				sweepTimedOutTasks(context.Background())
				require.NoError(t, model.DB.First(&persisted, task.ID).Error)
				assert.EqualValues(t, model.TaskStatusInProgress, persisted.Status)
				assert.True(t, persisted.PrivateData.ImageResultUnverified)
				assert.Equal(t, round+1, persisted.PrivateData.PollFailures)
				assert.Equal(t, "accepted-original-upstream", persisted.PrivateData.UpstreamTaskID)
				assert.Empty(t, persisted.PrivateData.ResultURL)
				assert.Empty(t, persisted.FailReason)
				assert.Zero(t, persisted.FinishTime)
				assert.Equal(t, preConsumed, persisted.Quota)
				assert.Equal(t, initialQuota, getUserQuota(t, userID))
				assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
				assert.Zero(t, countLogs(t))
			}

			// A late, verifiable result completes the same original task. The
			// initial reservation is retained and there is no second debit.
			var recovered model.Task
			require.NoError(t, model.DB.First(&recovered, task.ID).Error)
			accepted := &scriptedPollingAdaptor{parse: &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Url: "https://images.example/final.png"}}
			require.NoError(t, updateVideoSingleTask(context.Background(), accepted, ch, recovered.GetUpstreamTaskID(), map[string]*model.Task{recovered.GetUpstreamTaskID(): &recovered}))
			require.NoError(t, model.DB.First(&recovered, task.ID).Error)
			assert.EqualValues(t, model.TaskStatusSuccess, recovered.Status)
			assert.False(t, recovered.PrivateData.ImageResultUnverified)
			assert.Zero(t, recovered.PrivateData.PollFailures)
			assert.Equal(t, "https://images.example/final.png", recovered.PrivateData.ResultURL)
			assert.Equal(t, initialQuota, getUserQuota(t, userID))
			assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
			assert.Zero(t, countLogs(t))
		})
	}
}

func TestSunburstAcceptedConfirmedFailureRefundsOnceAfterUnknown(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 541, 541, 541
	const initialQuota, preConsumed, tokenRemain = 10000, 4000, 7000
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-sunburst-confirmed", tokenRemain)
	ch := &model.Channel{Id: channelID, Type: constant.ChannelTypeOpenAI, Name: "image-poll", Status: common.ChannelStatusEnabled}
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.TaskID, task.Action = "task_sunburst_confirmed", "influencer_sheet"
	task.Properties.OriginModelName = "gpt-image-2.5-sunburst"
	task.PrivateData.UpstreamTaskID = "accepted-original-failed"
	task.PrivateData.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "reference"}
	task.PrivateData.ImageResultUnverified = true
	require.NoError(t, model.DB.Create(task).Error)
	failed := &scriptedPollingAdaptor{parse: &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: "provider confirmed generation failed"}}
	require.NoError(t, updateVideoSingleTask(context.Background(), failed, ch, task.GetUpstreamTaskID(), map[string]*model.Task{task.GetUpstreamTaskID(): task}))
	var persisted model.Task
	require.NoError(t, model.DB.First(&persisted, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, persisted.Status)
	assert.False(t, persisted.PrivateData.ImageResultUnverified)
	assert.Zero(t, persisted.Quota)
	assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(1), countLogs(t))
	// Native refund idempotency retains the original receipt after replay.
	RefundTaskQuota(context.Background(), &persisted, persisted.FailReason)
	assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(1), countLogs(t))
}

func TestSunburstRecoveredUnloggedFailureUsesNativeReleaseWithoutDuplicateOrNegativeUsage(t *testing.T) {
	for _, fundingFails := range []bool{false, true} {
		name := "release_success"
		if fundingFails {
			name = "release_database_failure"
		}
		t.Run(name, func(t *testing.T) {
			truncate(t)
			const userID, channelID = 542, 542
			const initialQuota, reservation = 10000, 4000
			seedUser(t, userID, initialQuota)
			ch := &model.Channel{Id: channelID, Type: constant.ChannelTypeOpenAI, Name: "unlogged-image", Status: common.ChannelStatusEnabled}
			require.NoError(t, model.DB.Create(ch).Error)
			task := makeTask(userID, channelID, reservation, 0, BillingSourceWallet, 0)
			task.TaskID, task.Action, task.Platform = "task_unlogged_image", "influencer_sheet", "image_task_v1"
			task.Properties.OriginModelName = "gpt-image-2.5-sunburst"
			task.PrivateData.UpstreamTaskID = "real-original-failed"
			task.PrivateData.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text"}
			task.PrivateData.GenerationReservationUnlogged = true
			require.NoError(t, model.DB.Create(task).Error)
			if fundingFails {
				require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("test_image_release_db_failure", func(tx *gorm.DB) {
					if tx.Statement.Table == "users" {
						tx.AddError(errors.New("injected wallet database failure"))
					}
				}))
			}
			failed := &scriptedPollingAdaptor{parse: &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: "provider confirmed original failure"}}
			require.NoError(t, updateVideoSingleTask(context.Background(), failed, ch, task.GetUpstreamTaskID(), map[string]*model.Task{task.GetUpstreamTaskID(): task}))
			if fundingFails {
				require.NoError(t, model.DB.Callback().Update().Remove("test_image_release_db_failure"))
			}
			var persisted model.Task
			require.NoError(t, model.DB.First(&persisted, task.ID).Error)
			assert.EqualValues(t, model.TaskStatusFailure, persisted.Status)
			var user model.User
			require.NoError(t, model.DB.First(&user, userID).Error)
			assert.Zero(t, user.UsedQuota, "unlogged funding must not subtract consumption statistics")
			if fundingFails {
				assert.Equal(t, initialQuota, user.Quota)
				assert.Equal(t, reservation, persisted.Quota, "failed native release remains visible for funding reconciliation")
				assert.Zero(t, countLogs(t))
			} else {
				assert.Equal(t, initialQuota+reservation, user.Quota)
				assert.Zero(t, persisted.Quota)
				assert.Equal(t, int64(1), countLogs(t))
			}
			// No scheduler pass replays a terminal funding action, even after
			// the database becomes available again.
			oldFactory, oldLimit := GetTaskAdaptorFunc, constant.TaskQueryLimit
			GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return failed }
			constant.TaskQueryLimit = 100
			t.Cleanup(func() { GetTaskAdaptorFunc, constant.TaskQueryLimit = oldFactory, oldLimit })
			assert.Zero(t, RunTaskPollingOnce(context.Background(), nil).UnfinishedTasks)
			var after model.User
			require.NoError(t, model.DB.First(&after, userID).Error)
			assert.Equal(t, user.Quota, after.Quota)
			assert.Equal(t, user.UsedQuota, after.UsedQuota)
		})
	}
}
