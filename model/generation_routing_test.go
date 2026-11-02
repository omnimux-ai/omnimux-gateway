package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGenerationRoutingFixture(t *testing.T) (*Task, *Task, *SystemTask) {
	t.Helper()
	truncateTables(t)
	user := User{Username: "routing_user", AffCode: "routing_user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	scanner := &SystemTask{TaskID: "routing-scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "routing-runner", LockedUntil: common.GetTimestamp() + 300}).Error)
	task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: TaskStatusWaiting, Properties: Properties{Input: "original history"}, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{Body: []byte(`{"prompt":"preserved"}`), Routing: &GenerationRoutingPolicy{Mode: GenerationRoutingAuto, Groups: []string{"cheap", "backup"}, MaxAttempts: 2}}}}
	require.NoError(t, task.Insert())
	claimed, err := ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	started, err := BeginGenerationDispatch(context.Background(), claimed[0], scanner, "routing-runner")
	require.NoError(t, err)
	require.True(t, started)
	snapshot := &Task{UserId: user.Id, TaskID: task.TaskID, ChannelId: 15, Group: "cheap", Quota: 123, PrivateData: TaskPrivateData{BillingSource: "wallet", BillingContext: &TaskBillingContext{ModelPrice: 1, GroupRatio: 0.2, PerCallBilling: true}}}
	return claimed[0], snapshot, scanner
}

func TestGenerationRoutingPersistsOnlySafeBoundedAttempts(t *testing.T) {
	reserved, snapshot, scanner := newGenerationRoutingFixture(t)
	ctx := context.Background()
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	require.Len(t, reserved.PrivateData.GenerationQueue.Routing.Attempts, 1)
	assert.Equal(t, GenerationAttemptPending, reserved.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
	// The existing post-send snapshot is idempotent and never a new send.
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner"))
	assert.Len(t, reserved.PrivateData.GenerationQueue.Routing.Attempts, 1)
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	backup := *snapshot
	backup.ChannelId, backup.Group = 37, "backup"
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, &backup, scanner, "routing-runner", true), "an unanswered send must not be replayed through a backup")
	assert.Error(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "stale-runner", 15, GenerationAttemptRejected))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptRejected))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptRejected), "saving the same known result is idempotent")
	assert.Error(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptAccepted), "a late response cannot rewrite a safely rejected attempt")
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true), "a supplier shared by two groups must never be resent")
	wrongPrice := backup
	wrongPrice.Quota++
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, &wrongPrice, scanner, "routing-runner", true))
	wrongBilling := backup
	wrongBilling.PrivateData.BillingContext = &TaskBillingContext{ModelPrice: 2, GroupRatio: 0.2, PerCallBilling: true}
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, &wrongBilling, scanner, "routing-runner", true))
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, &backup, scanner, "routing-runner", true))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 37, GenerationAttemptRejected))
	third := backup
	third.ChannelId = 99
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, &third, scanner, "routing-runner", true), "cross-group retries cannot increase the frozen attempt budget")
	var saved Task
	require.NoError(t, DB.First(&saved, reserved.ID).Error)
	assert.Equal(t, TaskStatusDispatching, saved.Status)
	assert.Equal(t, reserved.TaskID, saved.TaskID)
	assert.Equal(t, "original history", saved.Properties.Input)
	assert.Equal(t, snapshot.Quota, saved.Quota)
	assert.Equal(t, snapshot.PrivateData.BillingContext, saved.PrivateData.BillingContext)
	assert.Equal(t, []GenerationRouteAttempt{{ChannelID: 15, Group: "cheap", Outcome: GenerationAttemptRejected}, {ChannelID: 37, Group: "backup", Outcome: GenerationAttemptRejected}}, saved.PrivateData.GenerationQueue.Routing.Attempts)
}

func TestGenerationRoutingUnknownAndRecoveryNeverEnableReplay(t *testing.T) {
	for _, outcome := range []string{GenerationAttemptPending, GenerationAttemptUnknown, GenerationAttemptAccepted} {
		t.Run(outcome, func(t *testing.T) {
			reserved, snapshot, scanner := newGenerationRoutingFixture(t)
			ctx := context.Background()
			require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
			if outcome != GenerationAttemptPending {
				require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, outcome))
			}
			require.NoError(t, RecoverGenerationClaims(ctx, scanner, "routing-runner"))
			var recovered Task
			require.NoError(t, DB.First(&recovered, reserved.ID).Error)
			assert.Equal(t, TaskStatusDispatching, recovered.Status)
			assert.Equal(t, outcome, recovered.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
			backup := *snapshot
			backup.ChannelId, backup.Group = 37, "backup"
			assert.Error(t, PersistGenerationSendSnapshot(ctx, &recovered, &backup, scanner, "routing-runner", true))
			require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
			assert.Error(t, PersistGenerationAttemptOutcome(ctx, &recovered, scanner, "routing-runner", 15, GenerationAttemptUnknown), "an expired lease must not finalize an attempt")
		})
	}
}

func TestGenerationRoutingRetainsReceiptAndAuditAfterQueueHandoff(t *testing.T) {
	reserved, snapshot, scanner := newGenerationRoutingFixture(t)
	inputs := &GenerationImageInputs{Version: 1, Mode: "masked_edit", Source: &GenerationImageReference{URL: "https://files.example/source.png", SHA256: "original-source"}, Mask: &GenerationImageReference{URL: "https://files.example/mask.png", SHA256: "original-mask"}}
	reserved.PrivateData.GenerationQueue.ImageInputs = inputs
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Update("private_data", reserved.PrivateData).Error)
	ctx := context.Background()
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	snapshot.PrivateData.UpstreamTaskID = "accepted-receipt"
	snapshot.PrivateData.PluginState = []byte(`{"receipt":"supplier-state"}`)
	snapshot.Data = []byte(`{"id":"accepted-receipt"}`)
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner"))
	conflicting := *snapshot
	conflicting.PrivateData.UpstreamTaskID = "different-receipt"
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, &conflicting, scanner, "routing-runner"))
	var pending Task
	require.NoError(t, DB.First(&pending, reserved.ID).Error)
	assert.Equal(t, TaskStatusDispatching, pending.Status)
	assert.Equal(t, snapshot.PrivateData.UpstreamTaskID, pending.PrivateData.UpstreamTaskID)
	assert.JSONEq(t, string(snapshot.PrivateData.PluginState), string(pending.PrivateData.PluginState))
	assert.JSONEq(t, string(snapshot.Data), string(pending.Data))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptAccepted))
	snapshot.Status = TaskStatusSubmitted
	require.NoError(t, PersistGenerationSubmission(ctx, snapshot, reserved))
	var submitted Task
	require.NoError(t, DB.First(&submitted, reserved.ID).Error)
	assert.EqualValues(t, TaskStatusSubmitted, submitted.Status)
	assert.Nil(t, submitted.PrivateData.GenerationQueue, "native polling must not keep an executable queue request")
	require.NotNil(t, submitted.PrivateData.GenerationRouting)
	assert.Equal(t, inputs, submitted.PrivateData.ImageInputs, "handoff retains immutable input audit without retaining an executable queue")
	assert.Equal(t, []GenerationRouteAttempt{{ChannelID: 15, Group: "cheap", Outcome: GenerationAttemptAccepted}}, submitted.PrivateData.GenerationRouting.Attempts)
}

func TestGenerationRoutingAcceptedReconciliationExpiryClaimsFundingOnce(t *testing.T) {
	reserved, snapshot, scanner := newGenerationRoutingFixture(t)
	ctx := context.Background()
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	snapshot.PrivateData.UpstreamTaskID = "accepted-needs-reconciliation"
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner"))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptAccepted))
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Update("start_time", 1).Error)
	expired, err := ExpireGenerationDispatches(ctx, scanner, "routing-runner", common.GetTimestamp(), 60, nil)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, snapshot.PrivateData.UpstreamTaskID, expired[0].PrivateData.UpstreamTaskID)
	assert.EqualValues(t, TaskStatusFailure, expired[0].Status)
	assert.Equal(t, GenerationAttemptAccepted, expired[0].PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
	again, err := ExpireGenerationDispatches(ctx, scanner, "routing-runner", common.GetTimestamp(), 60, nil)
	require.NoError(t, err)
	assert.Empty(t, again, "only one scanner can claim the reservation for native release")
	assert.Error(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptAccepted), "a late response cannot reopen an expired reservation")
}
