package model

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const influencerRequestTestScope = "influencer_creation:write"

func setupInfluencerRequestTest(t *testing.T) User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&InfluencerGenerationRequest{}))
	require.NoError(t, DB.Where("1 = 1").Delete(&InfluencerGenerationRequest{}).Error)
	truncateTables(t)
	t.Cleanup(func() { require.NoError(t, DB.Where("1 = 1").Delete(&InfluencerGenerationRequest{}).Error) })
	return createAccessTokenTestUser(t, "influencer-receipt-owner")
}

func influencerRequestTestTask(userID int) *Task {
	return &Task{TaskID: GenerateTaskID(), UserId: userID, Action: "influencer_sheet", Status: TaskStatusWaiting, Progress: "0%", PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{Path: "/pg/images/generations", Body: []byte(`{"prompt":"测试角色"}`)}}}
}

func TestInfluencerRequestAcceptanceAndReplay(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	ctx := context.Background()
	task := influencerRequestTestTask(user.Id)
	accepted, replay, err := CommitInfluencerGenerationRequest(ctx, task, "request-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	assert.False(t, replay)
	assert.Equal(t, task.TaskID, accepted.TaskID)
	assert.NotZero(t, task.ID)
	assert.Zero(t, accepted.Quota)

	var repeated *Task
	repeated, replay, err = CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "request-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	assert.True(t, replay)
	assert.Equal(t, accepted.ID, repeated.ID)
	_, _, err = CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "request-0001", strings.Repeat("b", 64), 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestConflict)
	var tasks, receipts int64
	require.NoError(t, DB.Model(&Task{}).Where("user_id = ?", user.Id).Count(&tasks).Error)
	require.NoError(t, DB.Model(&InfluencerGenerationRequest{}).Where("user_id = ?", user.Id).Count(&receipts).Error)
	assert.EqualValues(t, 1, tasks)
	assert.EqualValues(t, 1, receipts)

	receipt, err := GetInfluencerGenerationRequest(ctx, user.Id, "request-0001")
	require.NoError(t, err)
	assert.Equal(t, accepted.TaskID, receipt.TaskID)
	assert.Zero(t, receipt.RecoveryExpiresAt)
	_, err = GetInfluencerGenerationRequest(ctx, user.Id+1, "request-0001")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	other := createAccessTokenTestUser(t, "influencer-receipt-other")
	otherTask, replay, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(other.Id), "request-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	assert.False(t, replay)
	assert.NotEqual(t, accepted.TaskID, otherTask.TaskID)
}

func TestInfluencerRequestRevalidatesCurrentExplicitGrant(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, User, *UserAccessToken)
	}{
		{name: "revoked", prepare: func(t *testing.T, _ User, token *UserAccessToken) { require.NoError(t, DB.Delete(token).Error) }},
		{name: "expired", prepare: func(t *testing.T, _ User, token *UserAccessToken) {
			require.NoError(t, DB.Model(token).Update("expires_at", time.Now().Unix()).Error)
		}},
		{name: "scope removed", prepare: func(t *testing.T, _ User, token *UserAccessToken) {
			require.NoError(t, DB.Model(token).Update("scopes", `["influencer:read","usage:read"]`).Error)
		}},
		{name: "wildcard is not explicit", prepare: func(t *testing.T, _ User, token *UserAccessToken) {
			require.NoError(t, DB.Model(token).Update("scopes", `["influencer_creation:*"]`).Error)
		}},
		{name: "invalid stored scopes", prepare: func(t *testing.T, _ User, token *UserAccessToken) {
			require.NoError(t, DB.Model(token).Update("scopes", `{"influencer_creation:write":true}`).Error)
		}},
		{name: "foreign owner", prepare: func(t *testing.T, user User, token *UserAccessToken) {
			require.NoError(t, DB.Model(token).Update("user_id", user.Id+100).Error)
		}},
		{name: "disabled account", prepare: func(t *testing.T, user User, _ *UserAccessToken) {
			require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			user := setupInfluencerRequestTest(t)
			token := createTestUserAccessToken(t, user.Id, "nap_influencer_receipt_test", time.Now().Unix()+3600)
			require.NoError(t, token.SetScopes([]string{influencerRequestTestScope}))
			require.NoError(t, DB.Model(&token).Update("scopes", token.Scopes).Error)
			test.prepare(t, user, &token)
			_, _, err := CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), "token-check-0001", strings.Repeat("a", 64), token.Id, influencerRequestTestScope)
			assert.ErrorIs(t, err, ErrInfluencerRequestUnauthorized)
			var count int64
			require.NoError(t, DB.Model(&Task{}).Where("user_id = ?", user.Id).Count(&count).Error)
			assert.Zero(t, count)
		})
	}

	t.Run("current grant accepted", func(t *testing.T) {
		user := setupInfluencerRequestTest(t)
		token := createTestUserAccessToken(t, user.Id, "nap_influencer_current", time.Now().Unix()+3600)
		require.NoError(t, token.SetScopes([]string{influencerRequestTestScope}))
		require.NoError(t, DB.Model(&token).Update("scopes", token.Scopes).Error)
		accepted, _, err := CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), "token-accept-0001", strings.Repeat("a", 64), token.Id, influencerRequestTestScope)
		require.NoError(t, err)
		require.NoError(t, DB.Delete(&token).Error)
		_, _, err = CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), "token-accept-0001", strings.Repeat("a", 64), token.Id, influencerRequestTestScope)
		assert.ErrorIs(t, err, ErrInfluencerRequestUnauthorized)
		var task Task
		require.NoError(t, DB.First(&task, accepted.ID).Error)
		assert.Equal(t, TaskStatusWaiting, task.Status, "revocation must not delete or cancel already accepted work")
	})
}

func TestInfluencerRequestExpiryAndPermanentTombstone(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	ctx := context.Background()
	task, _, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "expiry-check-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&InfluencerGenerationRequest{}).Where("user_id = ?", user.Id).Update("created_at", time.Now().Add(-30*24*time.Hour).Unix()).Error)
	receipt, err := GetInfluencerGenerationRequest(ctx, user.Id, "expiry-check-0001")
	require.NoError(t, err, "active tasks do not expire")
	assert.Zero(t, receipt.RecoveryExpiresAt)

	finishedAt := time.Now().Add(-time.Hour).Unix()
	require.NoError(t, DB.Model(task).Updates(map[string]any{"status": TaskStatusSuccess, "finish_time": finishedAt}).Error)
	receipt, err = GetInfluencerGenerationRequest(ctx, user.Id, "expiry-check-0001")
	require.NoError(t, err)
	assert.Equal(t, finishedAt+7*24*60*60, receipt.RecoveryExpiresAt)
	// Persisting the expiry makes it independent of later task timestamp edits.
	require.NoError(t, DB.Model(task).Update("finish_time", time.Now().Unix()).Error)
	replayed, replay, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "expiry-check-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	assert.True(t, replay)
	assert.Equal(t, task.ID, replayed.ID)

	require.NoError(t, DB.Model(receipt).Update("recovery_expires_at", time.Now().Unix()-1).Error)
	expired, err := GetInfluencerGenerationRequest(ctx, user.Id, "expiry-check-0001")
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	require.NotNil(t, expired)
	assert.Equal(t, task.TaskID, expired.TaskID)
	_, _, err = CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "expiry-check-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	// A stale status rewrite cannot reopen a permanently expired request key.
	require.NoError(t, DB.Model(task).Update("status", TaskStatusWaiting).Error)
	_, _, err = CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "expiry-check-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	require.NoError(t, DB.Delete(task).Error)
	_, _, err = CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "expiry-check-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	var count int64
	require.NoError(t, DB.Model(&InfluencerGenerationRequest{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestInfluencerRequestMissingTaskAndUnstampedCompletion(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	ctx := context.Background()
	task, _, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "missing-task-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	require.NoError(t, DB.Model(task).Update("status", TaskStatusFailure).Error)
	first, err := GetInfluencerGenerationRequest(ctx, user.Id, "missing-task-0001")
	require.NoError(t, err)
	assert.Greater(t, first.RecoveryExpiresAt, time.Now().Unix())
	require.NoError(t, DB.Delete(task).Error)
	missing, err := GetInfluencerGenerationRequest(ctx, user.Id, "missing-task-0001")
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	require.NotNil(t, missing)
	assert.LessOrEqual(t, missing.RecoveryExpiresAt, time.Now().Unix())
	_, _, err = CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "missing-task-0001", strings.Repeat("b", 64), 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestConflict)
}

func TestInfluencerRequestUnavailableTaskCannotRecover(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	ctx := context.Background()
	task, _, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "discarded-task-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	task.PrivateData.ResultDiscarded = true
	require.NoError(t, DB.Model(task).Update("private_data", task.PrivateData).Error)
	receipt, err := GetInfluencerGenerationRequest(ctx, user.Id, "discarded-task-0001")
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	require.NotNil(t, receipt)
	assert.LessOrEqual(t, receipt.RecoveryExpiresAt, time.Now().Unix())
}

func TestInfluencerRequestUnstampedHistoricalCompletionExpires(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	ctx := context.Background()
	task, _, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "historical-task-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.NoError(t, err)
	require.NoError(t, DB.Model(task).UpdateColumns(map[string]any{"status": TaskStatusFailure, "updated_at": time.Now().Add(-8 * 24 * time.Hour).Unix()}).Error)
	receipt, err := GetInfluencerGenerationRequest(ctx, user.Id, "historical-task-0001")
	assert.ErrorIs(t, err, ErrInfluencerRequestExpired)
	require.NotNil(t, receipt)
	assert.Less(t, receipt.RecoveryExpiresAt, time.Now().Unix())
}

func TestInfluencerRequestCreationRollsBackBothRows(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	callbackName := "test:influencer_receipt_failure"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "influencer_generation_requests" {
			tx.AddError(errors.New("controlled receipt storage failure"))
		}
	}))
	defer DB.Callback().Create().Remove(callbackName)
	task := influencerRequestTestTask(user.Id)
	_, _, err := CommitInfluencerGenerationRequest(context.Background(), task, "rollback-check-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	require.Error(t, err)
	assert.Zero(t, task.ID, "a rolled back insertion must not make the caller believe it was accepted")
	for _, table := range []any{&Task{}, &InfluencerGenerationRequest{}} {
		var count int64
		require.NoError(t, DB.Model(table).Where("user_id = ?", user.Id).Count(&count).Error)
		assert.Zero(t, count)
	}
}

func TestInfluencerRequestRejectsInvalidAcceptance(t *testing.T) {
	user := setupInfluencerRequestTest(t)
	for _, requestID := range []string{"short", "spaces are invalid", "角色request0001", strings.Repeat("x", 129)} {
		_, _, err := CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), requestID, strings.Repeat("a", 64), 0, influencerRequestTestScope)
		assert.ErrorIs(t, err, ErrInfluencerRequestInvalid)
	}
	_, _, err := CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), "invalid-hash-0001", "not-a-sha256", 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestInvalid)
	_, _, err = CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), "invalid-token-0001", strings.Repeat("a", 64), -1, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestUnauthorized)
	_, _, err = CommitInfluencerGenerationRequest(context.Background(), influencerRequestTestTask(user.Id), "invalid-scope-0001", strings.Repeat("a", 64), 0, "")
	assert.ErrorIs(t, err, ErrInfluencerRequestUnauthorized)
	task := influencerRequestTestTask(user.Id)
	task.Status = TaskStatusSubmitted
	_, _, err = CommitInfluencerGenerationRequest(context.Background(), task, "invalid-task-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
	assert.ErrorIs(t, err, ErrInfluencerRequestInvalid)
}
