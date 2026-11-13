package model

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestGenerationQueueMixedFIFOAndAccountIsolation(t *testing.T) {
	truncateTables(t)
	user := User{Username: "queue_one", Status: common.UserStatusEnabled, AffCode: "queue_one"}
	other := User{Username: "queue_two", Status: common.UserStatusEnabled, AffCode: "queue_two"}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&other).Error)
	var ids []int64
	for i := 0; i < 6; i++ {
		action := "influencer_sheet"
		if i%2 == 1 {
			action = "influencer_motion"
		}
		task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: action, Status: TaskStatusWaiting, Progress: "0%", PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{Path: "/pg/video/generations", Body: []byte(`{"model":"seedance-2-5"}`)}}}
		require.NoError(t, task.Insert())
		ids = append(ids, task.ID)
	}
	require.NoError(t, (&Task{TaskID: GenerateTaskID(), UserId: other.Id, Action: "influencer_sheet", Status: TaskStatusWaiting, Progress: "0%", PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{}}}).Insert())
	var wg sync.WaitGroup
	results := make(chan []*Task, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tasks, err := ClaimGenerationTasks(context.Background(), user.Id, "owner")
			results <- tasks
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var claimed []*Task
	for batch := range results {
		claimed = append(claimed, batch...)
	}
	require.Len(t, claimed, 5)
	for i, task := range claimed {
		assert.Equal(t, ids[i], task.ID)
		assert.Zero(t, task.Quota)
	}
	isolated, err := ClaimGenerationTasks(context.Background(), other.Id, "owner")
	require.NoError(t, err)
	require.Len(t, isolated, 1)
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", ids[0]).Updates(map[string]any{"status": TaskStatusFailure, "progress": "100%"}).Error)
	next, err := ClaimGenerationTasks(context.Background(), user.Id, "owner")
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.Equal(t, ids[5], next[0].ID)
}

func TestGenerationQueueRecoveryDoesNotResendUnknownDispatch(t *testing.T) {
	truncateTables(t)
	user := User{Username: "recover_queue", AffCode: "recover_queue", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	for _, status := range []TaskStatus{TaskStatusClaimed, TaskStatusDispatching, TaskStatusWaiting} {
		require.NoError(t, (&Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: status, Progress: "0%", PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{ClaimOwner: "old"}}}).Insert())
	}
	scanner := &SystemTask{TaskID: "new_scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "new", LockedUntil: common.GetTimestamp() + 60}).Error)
	require.NoError(t, RecoverGenerationClaims(context.Background(), scanner, "new"))
	tasks, err := ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	var recovered Task
	require.NoError(t, DB.Where("user_id = ? AND status = ?", user.Id, TaskStatusDispatching).First(&recovered).Error)
	assert.Contains(t, recovered.FailReason, "待核对")
	for _, task := range tasks {
		started, err := BeginGenerationDispatch(context.Background(), task, scanner, "new")
		require.NoError(t, err)
		assert.True(t, started)
		var current Task
		require.NoError(t, DB.First(&current, task.ID).Error)
		assert.Empty(t, current.FailReason)
	}
	require.NoError(t, RecoverGenerationClaims(context.Background(), scanner, "new"))
	again, err := ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	assert.Empty(t, again)
	for _, task := range tasks {
		var current Task
		require.NoError(t, DB.First(&current, task.ID).Error)
		assert.Empty(t, current.FailReason, "current scanner's active preparation must not be marked unknown")
	}
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("type = ?", SystemTaskTypeGenerationQueue).Update("locked_by", "other").Error)
	assert.Error(t, RecoverGenerationClaims(context.Background(), scanner, "new"))
}

func TestGenerationQueueRestartMarksOnlyInterruptedDispatch(t *testing.T) {
	truncateTables(t)
	user := User{Username: "restart_queue", AffCode: "restart_queue", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	scanner := &SystemTask{TaskID: "current-scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "current-runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	var interrupted []*Task
	for _, mode := range []string{"preparing", "sent", "missing_worker", "active", "accepted"} {
		request := &GenerationQueueRequest{ClaimOwner: "old-scanner"}
		task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusDispatching, Quota: 5000, Properties: Properties{Input: "preserved"}, PrivateData: TaskPrivateData{GenerationQueue: request, BillingSource: "subscription", SubscriptionId: 77, SubscriptionAllowanceEpoch: 7, SubscriptionAllowanceAllocated: 5000}}
		switch mode {
		case "preparing":
			task.Quota = 0 // absence of a snapshot is not proof that no precharge occurred
		case "sent":
			request.SendStartedAt = common.GetTimestamp()
		case "missing_worker":
			task.PrivateData.GenerationQueue = nil
		case "active":
			request.ClaimOwner = scanner.TaskID
		case "accepted":
			task.Status = TaskStatusSuccess
			task.PrivateData.GenerationQueue = nil
		}
		require.NoError(t, task.Insert())
		if mode != "active" && mode != "accepted" {
			interrupted = append(interrupted, task)
		}
	}
	before := make([]Task, 0)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Order("id").Find(&before).Error)
	assert.Error(t, RecoverGenerationClaims(context.Background(), scanner, "wrong-runner"))
	var unchanged []Task
	require.NoError(t, DB.Where("user_id = ?", user.Id).Order("id").Find(&unchanged).Error)
	assert.Equal(t, before, unchanged, "reconciliation requires the native scanner lease")
	require.NoError(t, RecoverGenerationClaims(context.Background(), scanner, "current-runner"))
	for _, task := range before {
		var stored Task
		require.NoError(t, DB.First(&stored, task.ID).Error)
		if task.Status == TaskStatusSuccess || task.PrivateData.GenerationQueue != nil && task.PrivateData.GenerationQueue.ClaimOwner == scanner.TaskID {
			assert.Equal(t, task, stored)
			assert.Empty(t, stored.FailReason)
			continue
		}
		assert.Contains(t, stored.FailReason, "待核对")
		stored.FailReason = task.FailReason
		stored.UpdatedAt = task.UpdatedAt
		assert.Equal(t, task, stored, "recovery must preserve all funding and ownership fields")
	}
	waiting := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: TaskStatusWaiting, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{}}}
	require.NoError(t, waiting.Insert())
	active := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: TaskStatusDispatching, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{ClaimOwner: scanner.TaskID}}}
	require.NoError(t, active.Insert())
	next, err := ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	assert.Empty(t, next, "interrupted dispatch must retain its concurrency slot")
	for _, task := range interrupted {
		begun, err := BeginGenerationDispatch(context.Background(), task, scanner, "current-runner")
		require.NoError(t, err)
		assert.False(t, begun, "reconciliation must not restore an interrupted dispatch for resend")
	}
}

func TestGenerationQueueRecoveryPreservesSubmissionDiagnostic(t *testing.T) {
	truncateTables(t)
	user := User{Username: "diagnostic_queue", AffCode: "diagnostic_queue", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	reason := "提交结果待核对，不会自动重发（阶段：snapshot；代码：generation_snapshot_failed）"
	task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusDispatching, Quota: 5000, FailReason: reason, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{ClaimOwner: "old"}, BillingSource: "wallet"}}
	require.NoError(t, task.Insert())
	scanner := &SystemTask{TaskID: "diagnostic_scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	require.NoError(t, RecoverGenerationClaims(context.Background(), scanner, "runner"))
	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, reason, stored.FailReason)
	assert.Equal(t, TaskStatusDispatching, stored.Status)
	assert.Equal(t, 5000, stored.Quota)
}

func TestGenerationQueueExpireDispatches(t *testing.T) {
	truncateTables(t)
	user := User{Username: "expire_queue", AffCode: "expire_queue", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	scanner := &SystemTask{TaskID: "expire_scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	const cutoff int64 = 1000
	insert := func(owner string, startTime, sendStarted, releaseStarted int64, quota int, reason string) *Task {
		task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusDispatching, StartTime: startTime, Quota: quota, FailReason: reason,
			PrivateData: TaskPrivateData{BillingSource: "wallet", GenerationQueue: &GenerationQueueRequest{ClaimOwner: owner, SendStartedAt: sendStarted, ReleaseStartedAt: releaseStarted}}}
		require.NoError(t, task.Insert())
		return task
	}
	sent := insert("old", cutoff-1, cutoff-1, 0, 5000, "")
	preparing := insert("old", cutoff-1, 0, 0, 0, "")
	releaseBegun := insert("old", cutoff-1, cutoff-1, cutoff, 5000, "提交或资金释放待核对（阶段：submit；代码：submission_failed）")
	fresh := insert("old", cutoff+1, cutoff+1, 0, 5000, "")
	// The current scanner's own unconfirmed row expires once its worker ends;
	// only a still-running worker protects its row.
	ownFinished := insert(scanner.TaskID, cutoff-1, cutoff-1, 0, 5000, "")
	ownRunning := insert(scanner.TaskID, cutoff-1, cutoff-1, 0, 5000, "")
	waiting := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: TaskStatusWaiting, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{}}}
	require.NoError(t, waiting.Insert())
	inFlight := []int64{ownRunning.ID}

	_, err := ExpireGenerationDispatches(context.Background(), scanner, "stale-runner", cutoff, 60, inFlight)
	assert.Error(t, err, "expiry requires the current scanner lease")

	refundable, err := ExpireGenerationDispatches(context.Background(), scanner, "runner", cutoff, 60, inFlight)
	require.NoError(t, err)
	refundedIDs := []int64{}
	for _, task := range refundable {
		refundedIDs = append(refundedIDs, task.ID)
		assert.Equal(t, 5000, task.Quota)
	}
	assert.ElementsMatch(t, []int64{sent.ID, ownFinished.ID}, refundedIDs,
		"only a persisted reservation that no worker began to release is refundable")

	reload := func(task *Task) Task {
		var stored Task
		require.NoError(t, DB.First(&stored, task.ID).Error)
		return stored
	}
	for _, expired := range []*Task{sent, preparing, releaseBegun, ownFinished} {
		stored := reload(expired)
		assert.Equal(t, TaskStatus(TaskStatusFailure), stored.Status)
		assert.Equal(t, "100%", stored.Progress)
		assert.Positive(t, stored.FinishTime)
		assert.Equal(t, expired.Quota, stored.Quota, "expiry never settles funding itself")
		assert.Contains(t, stored.FailReason, "60")
	}
	assert.Contains(t, reload(sent).FailReason, "退回")
	assert.Contains(t, reload(preparing).FailReason, "人工核对")
	assert.Contains(t, reload(releaseBegun).FailReason, "人工核对")
	assert.Contains(t, reload(releaseBegun).FailReason, "submission_failed", "the original diagnostic is preserved")
	for _, kept := range []*Task{fresh, ownRunning} {
		assert.Equal(t, TaskStatusDispatching, reload(kept).Status)
	}

	again, err := ExpireGenerationDispatches(context.Background(), scanner, "runner", cutoff, 60, inFlight)
	require.NoError(t, err)
	assert.Empty(t, again, "an expired dispatch is never returned twice")
	claimed, err := ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1, "expired dispatches release their concurrency slots")
	assert.Equal(t, waiting.ID, claimed[0].ID)
}

// A worker may release its reservation only while its row is still the same
// DISPATCHING claim; once expiry has failed (and refunded) it, release is refused.
func TestGenerationQueueReleaseMarkerFencesExpiry(t *testing.T) {
	truncateTables(t)
	user := User{Username: "release_queue", AffCode: "release_queue", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	scanner := &SystemTask{TaskID: "release_scanner"}
	require.NoError(t, DB.Create(&SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	newRow := func() *Task {
		task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusDispatching, StartTime: 1, Quota: 5000,
			PrivateData: TaskPrivateData{BillingSource: "wallet", GenerationQueue: &GenerationQueueRequest{ClaimOwner: "old", SendStartedAt: 1}}}
		require.NoError(t, task.Insert())
		return task
	}

	claimed := newRow()
	require.NoError(t, MarkGenerationReleaseStarted(context.Background(), claimed))
	refundable, err := ExpireGenerationDispatches(context.Background(), scanner, "runner", 1000, 60, nil)
	require.NoError(t, err)
	assert.Empty(t, refundable, "a row whose worker began releasing is never refunded by expiry")

	expired := newRow()
	refundable, err = ExpireGenerationDispatches(context.Background(), scanner, "runner", 1000, 60, nil)
	require.NoError(t, err)
	require.Len(t, refundable, 1)
	assert.Equal(t, expired.ID, refundable[0].ID)
	assert.Error(t, MarkGenerationReleaseStarted(context.Background(), expired), "a late worker cannot release a row expiry already refunded")

	reowned := newRow()
	other := *reowned
	other.PrivateData.GenerationQueue = &GenerationQueueRequest{ClaimOwner: "someone-else"}
	assert.Error(t, MarkGenerationReleaseStarted(context.Background(), &other), "release requires the same claim owner")
}

func TestGenerationQueueDisabledUserCannotStart(t *testing.T) {
	truncateTables(t)
	user := User{Username: "disabled_queue", AffCode: "disabled_queue", Status: common.UserStatusDisabled}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, (&Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: TaskStatusWaiting, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{}}}).Insert())
	tasks, err := ClaimGenerationTasks(context.Background(), user.Id, "scanner")
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestGenerationQueueDoesNotPollOrTimeOutLocalTasks(t *testing.T) {
	truncateTables(t)
	for _, status := range []TaskStatus{"WAITING", "CLAIMED", "DISPATCHING"} {
		require.NoError(t, (&Task{TaskID: string(status), Status: status, Progress: "0%", SubmitTime: 1}).Insert())
	}
	assert.Empty(t, GetAllUnFinishSyncTasks(100))
	assert.Empty(t, GetTimedOutUnfinishedTasks(100, 100))
	assert.False(t, HasUnfinishedSyncTasks())
}
