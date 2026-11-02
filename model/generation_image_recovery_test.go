package model

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSunburstVersionedDispatchExpiryPreservesUncertainFunds(t *testing.T) {
	for _, outcome := range []string{GenerationAttemptUnknown, GenerationAttemptAccepted} {
		t.Run(outcome, func(t *testing.T) {
			reserved, snapshot, scanner := newGenerationRoutingFixture(t)
			reserved.Properties.OriginModelName = "gpt-image-2.5-sunburst"
			reserved.PrivateData.GenerationQueue.ImageInputs = &GenerationImageInputs{Version: 1, Mode: "text"}
			require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Updates(map[string]any{"properties": reserved.Properties, "private_data": reserved.PrivateData}).Error)
			snapshot.Properties.OriginModelName = reserved.Properties.OriginModelName
			snapshot.Platform = constant.TaskPlatform("image_task_v1")
			ctx := context.Background()
			require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
			if outcome == GenerationAttemptAccepted {
				snapshot.PrivateData.UpstreamTaskID = "accepted-before-crash"
				snapshot.Data = []byte(`{"id":"accepted-before-crash","status":"pending"}`)
				require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner"))
			}
			require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, outcome))
			require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Update("start_time", common.GetTimestamp()-61*60).Error)
			refundable, err := ExpireGenerationDispatches(ctx, scanner, "routing-runner", common.GetTimestamp()-60*60, 60, nil)
			require.NoError(t, err)
			var saved Task
			require.NoError(t, DB.First(&saved, reserved.ID).Error)
			t.Logf("observed outcome=%s status=%s refundable=%d quota=%d upstream=%s reason=%s", outcome, saved.Status, len(refundable), saved.Quota, saved.PrivateData.UpstreamTaskID, saved.FailReason)
			assert.Empty(t, refundable, "elapsed time is not proof that a Sunburst task was rejected")
			assert.NotEqual(t, TaskStatus(TaskStatusFailure), saved.Status, "uncertain/accepted task must remain reconcilable")
		})
	}
}

func TestSunburstAcceptedReceiptSurvivesCrashAndReachesNativePoller(t *testing.T) {
	reserved, snapshot, scanner := newGenerationRoutingFixture(t)
	reserved.Properties.OriginModelName = "gpt-image-2.5-sunburst"
	reserved.PrivateData.GenerationQueue.ImageInputs = &GenerationImageInputs{Version: 1, Mode: "text"}
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Updates(map[string]any{"properties": reserved.Properties, "private_data": reserved.PrivateData}).Error)
	snapshot.Properties.OriginModelName = reserved.Properties.OriginModelName
	snapshot.Platform = constant.TaskPlatform("image_task_v1")
	ctx := context.Background()
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	snapshot.PrivateData.UpstreamTaskID = "accepted-before-handoff-crash"
	snapshot.Data = []byte(`{"id":"accepted-before-handoff-crash","status":"pending"}`)
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner"))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptAccepted))
	require.NoError(t, RecoverGenerationClaims(ctx, scanner, "routing-runner"))
	rows := GetAllUnFinishSyncTasks(100)
	found := false
	for _, row := range rows {
		if row.ID == reserved.ID {
			found = true
		}
	}
	var saved Task
	require.NoError(t, DB.First(&saved, reserved.ID).Error)
	t.Logf("observed status=%s saved-upstream=%s poller-count=%d", saved.Status, saved.PrivateData.UpstreamTaskID, len(rows))
	assert.True(t, found, "persisted accepted receipt must reach original-task GET after a crash")
}

func TestSunburstLateReceiptClosesUnknownAtomicallyWithoutAnotherAttempt(t *testing.T) {
	reserved, snapshot, scanner := newGenerationRoutingFixture(t)
	reserved.Properties.OriginModelName = "gpt-image-2.5-sunburst"
	reserved.PrivateData.GenerationQueue.ImageInputs = &GenerationImageInputs{Version: 1, Mode: "text"}
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Updates(map[string]any{"properties": reserved.Properties, "private_data": reserved.PrivateData}).Error)
	snapshot.Properties.OriginModelName, snapshot.Platform = reserved.Properties.OriginModelName, "image_task_v1"
	ctx := context.Background()
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptUnknown))
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Update("start_time", common.GetTimestamp()-61*60).Error)
	refundable, err := ExpireGenerationDispatches(ctx, scanner, "routing-runner", common.GetTimestamp()-60*60, 60, nil)
	require.NoError(t, err)
	assert.Empty(t, refundable)
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true), "unknown may be reconciled but never sent again")
	snapshot.PrivateData.UpstreamTaskID = reserved.TaskID // synchronous response uses a synthetic local ID
	snapshot.PrivateData.GenerationAcceptedResult = &GenerationAcceptedResult{Status: TaskStatusSuccess, URL: "https://images.example/original.png", Progress: "100%"}
	snapshot.Data = []byte(`{"data":[{"url":"https://gateway.example/proxy"}]}`)
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner"))
	var accepted Task
	require.NoError(t, DB.First(&accepted, reserved.ID).Error)
	assert.Equal(t, GenerationAttemptAccepted, accepted.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome, "receipt and accepted outcome share the same transaction")
	assert.Len(t, accepted.PrivateData.GenerationQueue.Routing.Attempts, 1)
	assert.Equal(t, "https://images.example/original.png", accepted.PrivateData.ResultURL)
	conflicting := *snapshot
	conflicting.PrivateData.GenerationAcceptedResult = &GenerationAcceptedResult{Status: TaskStatusSuccess, URL: "https://images.example/different.png"}
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, &conflicting, scanner, "routing-runner"))
	require.NoError(t, RecoverGenerationClaims(ctx, scanner, "routing-runner"))
	require.NoError(t, DB.First(&accepted, reserved.ID).Error)
	assert.EqualValues(t, TaskStatusSubmitted, accepted.Status)
	assert.Nil(t, accepted.PrivateData.GenerationQueue)
	assert.Equal(t, "https://images.example/original.png", accepted.PrivateData.ResultURL)
	assert.Equal(t, snapshot.Quota, accepted.Quota)
	assert.Error(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
}

func TestSunburstKnownRejectedDispatchKeepsNativeExpiryRelease(t *testing.T) {
	reserved, snapshot, scanner := newGenerationRoutingFixture(t)
	reserved.Properties.OriginModelName = "gpt-image-2.5-sunburst"
	reserved.PrivateData.GenerationQueue.ImageInputs = &GenerationImageInputs{Version: 1, Mode: "text"}
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Updates(map[string]any{"properties": reserved.Properties, "private_data": reserved.PrivateData}).Error)
	snapshot.Properties.OriginModelName, snapshot.Platform = reserved.Properties.OriginModelName, "image_task_v1"
	ctx := context.Background()
	require.NoError(t, PersistGenerationSendSnapshot(ctx, reserved, snapshot, scanner, "routing-runner", true))
	require.NoError(t, PersistGenerationAttemptOutcome(ctx, reserved, scanner, "routing-runner", 15, GenerationAttemptRejected))
	require.NoError(t, RecoverGenerationClaims(ctx, scanner, "routing-runner"))
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", reserved.ID).Update("start_time", 1).Error)
	released, err := ExpireGenerationDispatches(ctx, scanner, "routing-runner", common.GetTimestamp(), 60, nil)
	require.NoError(t, err)
	require.Len(t, released, 1)
	assert.EqualValues(t, TaskStatusFailure, released[0].Status)
	assert.False(t, released[0].PrivateData.ImageResultUnverified)
	assert.Equal(t, snapshot.Quota, released[0].Quota)
	again, err := ExpireGenerationDispatches(ctx, scanner, "routing-runner", common.GetTimestamp(), 60, nil)
	require.NoError(t, err)
	assert.Empty(t, again)
}
