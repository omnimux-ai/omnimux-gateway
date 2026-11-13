package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGenerationObjectSwapFixture(t *testing.T) (*Task, *SystemTask) {
	t.Helper()
	truncateTables(t)
	user := User{Username: "swap_preparation", AffCode: "swap_preparation", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	scanner := &SystemTask{TaskID: "swap-scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "swap-runner", LockedUntil: common.GetTimestamp() + 300}).Error)
	task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusWaiting, Properties: Properties{Input: "original request"}, PrivateData: TaskPrivateData{Key: "existing-private-data", GenerationQueue: &GenerationQueueRequest{Body: []byte(`{"prompt":"original","videos":["source"],"images":["image1","image2"]}`), Routing: &GenerationRoutingPolicy{Mode: GenerationRoutingAuto, Groups: []string{"video-supplier"}, MaxAttempts: 1}}}}
	require.NoError(t, task.Insert())
	claimed, err := ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	return claimed[0], scanner
}

func TestGenerationObjectSwapPreparationCachesReadyAndSurvivesNativeHandoff(t *testing.T) {
	task, scanner := newGenerationObjectSwapFixture(t)
	ctx := context.Background()
	queue := *task.PrivateData.GenerationQueue
	preparation, begun, err := BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
	require.NoError(t, err)
	require.True(t, begun)
	require.Equal(t, GenerationObjectSwapRunning, preparation.State)
	require.Positive(t, preparation.StartedAt)
	startedAt := preparation.StartedAt
	details := []byte(`{"mapping":[{"images":[1,2],"target":"original target"}],"usage":null,"request_id":"request-evidence"}`)
	result := GenerationObjectSwapPreparation{State: GenerationObjectSwapReady, StartedAt: 1, FinishedAt: 1, Prompt: "Resolved object replacement", Details: details}
	require.NoError(t, FinishGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner", result))
	assert.Equal(t, startedAt, task.PrivateData.ObjectSwapPreparation.StartedAt)
	assert.Greater(t, task.PrivateData.ObjectSwapPreparation.FinishedAt, int64(1))
	assert.Equal(t, "existing-private-data", task.PrivateData.Key)
	assert.Equal(t, queue, *task.PrivateData.GenerationQueue)

	// Re-read from storage through Begin, rather than trusting the first worker's
	// in-memory result. A completed helper must not consume another call.
	preparation, begun, err = BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
	require.NoError(t, err)
	assert.False(t, begun)
	assert.Equal(t, result.Prompt, preparation.Prompt)
	assert.JSONEq(t, string(details), string(preparation.Details))
	assert.Error(t, FinishGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner", result), "a late completion cannot overwrite a final result")
	newScanner := &SystemTask{TaskID: "ready-replacement-scanner"}
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("type = ?", SystemTaskTypeGenerationQueue).Update("task_id", newScanner.TaskID).Error)
	require.NoError(t, RecoverGenerationClaims(ctx, newScanner, "swap-runner"))
	reclaimed, err := ClaimGenerationTasks(ctx, task.UserId, newScanner.TaskID)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	task, scanner = reclaimed[0], newScanner
	preparation, begun, err = BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
	require.NoError(t, err)
	assert.False(t, begun, "a recovered claim reuses the accepted preparation")
	assert.Equal(t, result.Prompt, preparation.Prompt)

	dispatched, err := BeginGenerationDispatch(ctx, task, scanner, "swap-runner")
	require.NoError(t, err)
	require.True(t, dispatched)
	native := &Task{TaskID: task.TaskID, UserId: task.UserId, Status: TaskStatusSubmitted, PrivateData: TaskPrivateData{UpstreamTaskID: "accepted-video"}}
	require.NoError(t, PersistGenerationSubmission(ctx, native, task))
	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Nil(t, stored.PrivateData.GenerationQueue)
	require.NotNil(t, stored.PrivateData.ObjectSwapPreparation)
	assert.Equal(t, result.Prompt, stored.PrivateData.ObjectSwapPreparation.Prompt)
	assert.JSONEq(t, string(details), string(stored.PrivateData.ObjectSwapPreparation.Details))
	assert.Equal(t, "original request", stored.Properties.Input)
}

func TestGenerationObjectSwapPreparationCrashRecoveryDoesNotReplay(t *testing.T) {
	task, scanner := newGenerationObjectSwapFixture(t)
	ctx := context.Background()
	preparation, begun, err := BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
	require.NoError(t, err)
	require.True(t, begun)
	startedAt := preparation.StartedAt

	newScanner := &SystemTask{TaskID: "replacement-scanner"}
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("type = ?", SystemTaskTypeGenerationQueue).Updates(map[string]any{"task_id": newScanner.TaskID, "locked_by": "replacement-runner"}).Error)
	require.NoError(t, RecoverGenerationClaims(ctx, newScanner, "replacement-runner"))
	claimed, err := ClaimGenerationTasks(ctx, task.UserId, newScanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Error(t, FinishGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner", GenerationObjectSwapPreparation{State: GenerationObjectSwapReady, Prompt: "late response", Details: []byte(`{"usage":{"total_tokens":30}}`)}))
	preparation, begun, err = BeginGenerationObjectSwapPreparation(ctx, claimed[0], newScanner, "replacement-runner")
	require.NoError(t, err)
	assert.False(t, begun, "a missing response does not prove that the paid helper was never accepted")
	assert.Equal(t, GenerationObjectSwapUnknown, preparation.State)
	assert.Equal(t, startedAt, preparation.StartedAt)
	assert.Positive(t, preparation.FinishedAt)
	assert.Empty(t, preparation.Prompt)
	assert.Error(t, FinishGenerationObjectSwapPreparation(ctx, claimed[0], newScanner, "replacement-runner", GenerationObjectSwapPreparation{State: GenerationObjectSwapReady, Prompt: "late response", Details: []byte(`{}`)}))
	preparation, begun, err = BeginGenerationObjectSwapPreparation(ctx, claimed[0], newScanner, "replacement-runner")
	require.NoError(t, err)
	assert.False(t, begun)
	assert.Equal(t, GenerationObjectSwapUnknown, preparation.State)
}

func TestGenerationObjectSwapPreparationFailurePreservesUsageEvidenceAndStops(t *testing.T) {
	for _, state := range []string{GenerationObjectSwapFailed, GenerationObjectSwapUnknown} {
		t.Run(state, func(t *testing.T) {
			task, scanner := newGenerationObjectSwapFixture(t)
			ctx := context.Background()
			_, begun, err := BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
			require.NoError(t, err)
			require.True(t, begun)
			details := []byte(`{"error":"response_invalid","usage":null,"provider_request_id":"request-with-unknown-usage"}`)
			require.NoError(t, FinishGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner", GenerationObjectSwapPreparation{State: state, Details: details}))
			preparation, begun, err := BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
			require.NoError(t, err)
			assert.False(t, begun)
			assert.Equal(t, state, preparation.State)
			assert.Empty(t, preparation.Prompt)
			assert.JSONEq(t, string(details), string(preparation.Details))
		})
	}
}

func TestGenerationObjectSwapPreparationFencesAccountClaimAndLease(t *testing.T) {
	for _, scenario := range []string{"expired lease", "different runner", "different claim", "different account", "different task id", "disabled account", "submission started", "send already recorded"} {
		t.Run(scenario, func(t *testing.T) {
			task, scanner := newGenerationObjectSwapFixture(t)
			ctx := context.Background()
			_, begun, err := BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
			require.NoError(t, err)
			require.True(t, begun)
			runner := "swap-runner"
			switch scenario {
			case "expired lease":
				require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
			case "different runner":
				runner = "wrong-runner"
			case "different claim":
				private := task.PrivateData
				queue := *private.GenerationQueue
				queue.ClaimOwner = "new-owner"
				private.GenerationQueue = &queue
				require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).Update("private_data", private).Error)
			case "different account":
				other := User{Username: "other_swap_owner", AffCode: "other_swap_owner", Status: common.UserStatusEnabled}
				require.NoError(t, DB.Create(&other).Error)
				task.UserId = other.Id
			case "different task id":
				task.TaskID = "wrong-task-id"
			case "disabled account":
				require.NoError(t, DB.Model(&User{}).Where("id = ?", task.UserId).Update("status", common.UserStatusDisabled).Error)
			case "submission started":
				require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).Update("status", TaskStatusDispatching).Error)
			case "send already recorded":
				private := task.PrivateData
				queue := *private.GenerationQueue
				queue.SendStartedAt = common.GetTimestamp()
				private.GenerationQueue = &queue
				require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).Update("private_data", private).Error)
			}
			_, begun, err = BeginGenerationObjectSwapPreparation(ctx, task, scanner, runner)
			assert.Error(t, err)
			assert.False(t, begun)
			assert.Error(t, FinishGenerationObjectSwapPreparation(ctx, task, scanner, runner, GenerationObjectSwapPreparation{State: GenerationObjectSwapReady, Prompt: "forbidden completion", Details: []byte(`{}`)}))
			var stored Task
			require.NoError(t, DB.First(&stored, task.ID).Error)
			assert.Equal(t, GenerationObjectSwapRunning, stored.PrivateData.ObjectSwapPreparation.State)
		})
	}
}

func TestGenerationObjectSwapPreparationRejectsInvalidCompletion(t *testing.T) {
	task, scanner := newGenerationObjectSwapFixture(t)
	ctx := context.Background()
	_, begun, err := BeginGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner")
	require.NoError(t, err)
	require.True(t, begun)
	for _, result := range []GenerationObjectSwapPreparation{
		{State: GenerationObjectSwapRunning},
		{State: GenerationObjectSwapReady, Details: []byte(`{}`)},
		{State: GenerationObjectSwapReady, Prompt: "missing audit"},
		{State: GenerationObjectSwapReady, Prompt: "invalid audit", Details: []byte(`not json`)},
		{State: GenerationObjectSwapReady, Prompt: "non-object audit", Details: []byte(`[]`)},
		{State: GenerationObjectSwapFailed, Prompt: "failure cannot become video instructions"},
	} {
		assert.Error(t, FinishGenerationObjectSwapPreparation(ctx, task, scanner, "swap-runner", result))
	}
	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, GenerationObjectSwapRunning, stored.PrivateData.ObjectSwapPreparation.State)
}
