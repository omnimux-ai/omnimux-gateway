package controller

import (
	"context"
	"errors"
	"gorm.io/gorm/clause"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGenerationQueueNativeValidationAndBuildReleaseSlot(t *testing.T) {
	for _, mode := range []string{"validation", "build"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }), func(s string) string {
				if mode == "validation" {
					return strings.Replace(s, `return {url:ctx.baseUrl+"/submit",body:ctx.requestBody};`, `throw new Error("local validation refusal");`, 1)
				}
				return strings.Replace(s, `body:ctx.requestBody`, `body:{file:{__fileRef:"missing:0",encoding:"base64"}}`, 1)
			})
			task := insertCorrectionTask(t, user, 1, false)
			for i := 2; i <= 5; i++ {
				active := insertCorrectionTask(t, user, i, false)
				require.NoError(t, db.Model(active).Update("status", model.TaskStatusDispatching).Error)
			}
			sixth := insertCorrectionTask(t, user, 6, false)
			var sawPrecharge atomic.Bool
			require.NoError(t, db.Callback().Update().After("gorm:update").Register("observe_precharge", func(tx *gorm.DB) {
				if tx.Statement.Table == "users" {
					if values, ok := tx.Statement.Dest.(map[string]interface{}); ok {
						if expr, ok := values["quota"].(clause.Expr); ok && strings.Contains(strings.ReplaceAll(expr.SQL, " ", ""), "quota-?") {
							sawPrecharge.Store(true)
						}
					}
				}
			}))
			claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
			stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
			assert.Zero(t, calls.Load())
			assert.Zero(t, stored.Quota)
			account, err := model.GetUserById(user.Id, false)
			require.NoError(t, err)
			assert.Equal(t, 100000, account.Quota)
			assert.Zero(t, account.UsedQuota)
			if mode == "build" {
				assert.True(t, sawPrecharge.Load(), "real body failure must occur after native precharge")
			}
			next, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
			require.NoError(t, err)
			require.Len(t, next, 1)
			assert.Equal(t, sixth.ID, next[0].ID)
		})
	}
}

func TestGenerationQueueSnapshotFailureNeverSendsAndRefunds(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	task := insertCorrectionTask(t, user, 1, false)
	var precharged atomic.Bool
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fail_send_snapshot", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]interface{})
		if tx.Statement.Table != "tasks" || !ok {
			return
		}
		if quota, ok := values["quota"].(int); ok && quota > 0 {
			var account model.User
			if tx.Session(&gorm.Session{NewDB: true}).First(&account, user.Id).Error == nil {
				precharged.Store(account.Quota == 95000)
			}
			tx.AddError(errors.New("snapshot write rejected"))
		}
	}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	assert.True(t, precharged.Load())
	assert.Zero(t, calls.Load())
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Zero(t, stored.Quota)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
}

func TestGenerationQueueProviderObservesCommittedSnapshotAndSameSuccessRow(t *testing.T) {
	observed := make(chan *model.Task, 1)
	var db *gorm.DB
	var task *model.Task
	var user model.User
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var row model.Task
		if err := db.Session(&gorm.Session{NewDB: true}).First(&row, task.ID).Error; err != nil {
			http.Error(w, "missing snapshot", 500)
			return
		}
		observed <- &row
		_, _ = w.Write([]byte(`{"id":"vendor-only","status":"SUCCESS"}`))
	}))
	task = insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	snapshot := <-observed
	assert.Equal(t, 5000, snapshot.Quota)
	assert.Equal(t, "wallet", snapshot.PrivateData.BillingSource)
	assert.Equal(t, 1, snapshot.ChannelId)
	assert.Equal(t, "default", snapshot.Group)
	assert.Equal(t, "queue-correction", snapshot.Properties.UpstreamModelName)
	assert.Equal(t, task.Properties.Input, snapshot.Properties.Input)
	require.NotNil(t, snapshot.PrivateData.Execution)
	assert.NotEmpty(t, snapshot.PrivateData.Execution.RequestID)
	require.NotNil(t, snapshot.PrivateData.BillingContext)
	assert.Equal(t, "queue-correction", snapshot.PrivateData.BillingContext.OriginModelName)
	assert.NotZero(t, snapshot.PrivateData.GenerationQueue.SendStartedAt)
	assert.Zero(t, snapshot.PrivateData.TokenId)
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, task.ID, stored.ID)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
	assert.Empty(t, stored.FailReason)
	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGenerationQueueLateOwnerAtGateNeverSends(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	task := insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	var replaced atomic.Bool
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("replace_owner_after_precharge", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]interface{})
		if tx.Statement.Table != "users" || !ok {
			return
		}
		expr, ok := values["quota"].(clause.Expr)
		if !ok || !strings.Contains(expr.SQL, "quota -") || replaced.Swap(true) {
			return
		}
		var row model.Task
		conn := tx.Session(&gorm.Session{NewDB: true})
		if err := conn.First(&row, task.ID).Error; err != nil {
			tx.AddError(err)
			return
		}
		row.PrivateData.GenerationQueue.ClaimOwner = "new-owner"
		tx.AddError(conn.Model(&row).Update("private_data", row.PrivateData).Error)
	}))
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	assert.True(t, replaced.Load())
	assert.Zero(t, calls.Load())
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Equal(t, "new-owner", stored.PrivateData.GenerationQueue.ClaimOwner)
	assert.Zero(t, stored.PrivateData.GenerationQueue.SendStartedAt)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota)
}

func TestGenerationQueueHTTP400RefundFailureKeepsSnapshot(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(400) }))
	task := insertCorrectionTask(t, user, 1, false)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("refuse_native_refund", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]interface{})
		if tx.Statement.Table == "users" && ok {
			if expr, ok := values["quota"].(clause.Expr); ok && strings.Contains(expr.SQL, "quota +") {
				tx.AddError(errors.New("native refund refused"))
			}
		}
	}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Equal(t, 5000, stored.Quota)
	require.NotNil(t, stored.PrivateData.BillingContext)
	assert.Contains(t, stored.FailReason, "待核对")
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 95000, account.Quota)
	assert.Equal(t, int32(1), calls.Load())
}

func TestGenerationQueueReserveRefreshesSnapshotBeforeResultFailure(t *testing.T) {
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"upward","status":"SUCCESS"}`))
	}), func(s string) string {
		return strings.Replace(s, `export function extractUsage(){return {units:1};}`, `export function extractUsage(){return {units:1};}export function extractUsageOnComplete(){return {units:2};}`, 1)
	})
	task := insertCorrectionTask(t, user, 1, false)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fail_upward_result", func(tx *gorm.DB) {
		if result, ok := tx.Statement.Dest.(*model.Task); ok && result.Status == model.TaskStatusSuccess {
			tx.AddError(errors.New("result persistence rejected"))
		}
	}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Equal(t, 10000, stored.Quota)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 90000, account.Quota)
}

func TestGenerationQueueAcceptedResponseSnapshotCancellationKeepsDiagnostic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"accepted-before-cancellation","status":"SUCCESS"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	updates := 0
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("cancel_after_second_snapshot", func(tx *gorm.DB) {
		if fields, ok := tx.Statement.Dest.(map[string]any); ok {
			if _, snapshot := fields["private_data"]; snapshot && fields["platform"] != nil {
				updates++
				if updates == 2 {
					cancel()
				}
			}
		}
	}))
	claimed, err := model.ClaimGenerationTasks(ctx, user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(ctx, claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Contains(t, stored.FailReason, "阶段：snapshot；代码：generation_snapshot_failed")
	assert.Equal(t, 5000, stored.Quota)
}

func TestGenerationQueueResultPersistenceFailureRetainsNativeFunding(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"id":"vendor-only","status":"SUCCESS"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fail_result", func(tx *gorm.DB) {
		if result, ok := tx.Statement.Dest.(*model.Task); ok && result.Status == model.TaskStatusSuccess {
			tx.AddError(errors.New("result write rejected"))
		}
	}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Equal(t, 5000, stored.Quota)
	require.NotNil(t, stored.PrivateData.BillingContext)
	assert.Contains(t, stored.FailReason, "待核对")
	assert.Contains(t, stored.FailReason, "阶段：insert；代码：task_insert_failed")
	assert.Equal(t, int32(1), calls.Load())
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 95000, account.Quota)
	assert.Zero(t, account.UsedQuota)
}

func TestGenerationQueueSubscriptionSnapshotUnknownAndHTTP400(t *testing.T) {
	for _, mode := range []string{"unknown", "http400", "zero_estimate", "build_failure"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if mode == "http400" {
					w.WriteHeader(400)
				} else {
					_, _ = w.Write([]byte(`{"unexpected":"accepted"}`))
				}
			}), func(s string) string {
				if mode == "build_failure" {
					return strings.Replace(s, `body:ctx.requestBody`, `body:{file:{__fileRef:"missing:0",encoding:"base64"}}`, 1)
				}
				return s
			})
			require.NoError(t, db.Model(&user).Update("setting", `{"billing_preference":"subscription_only"}`).Error)
			plan := model.SubscriptionPlan{Title: "test native plan", TotalAmount: 100000}
			require.NoError(t, db.Create(&plan).Error)
			sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: "active", StartTime: common.GetTimestamp() - 1, EndTime: common.GetTimestamp() + 3600, AllowanceEpoch: 7}
			require.NoError(t, db.Create(&sub).Error)
			if mode == "zero_estimate" {
				withTieredBillingConfig(t, map[string]string{"queue-correction": billing_setting.BillingModeTieredExpr}, map[string]string{"queue-correction": `u("units") * 0`})
			}
			task := insertCorrectionTask(t, user, 1, false)
			claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
			stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
			require.NoError(t, err)
			require.True(t, found)
			require.NoError(t, db.First(&sub, sub.Id).Error)
			assert.Equal(t, "subscription", stored.PrivateData.BillingSource)
			assert.Equal(t, sub.Id, stored.PrivateData.SubscriptionId)
			assert.Equal(t, int64(7), stored.PrivateData.SubscriptionAllowanceEpoch)
			if mode != "build_failure" {
				require.NotNil(t, stored.PrivateData.Execution)
				assert.NotEmpty(t, stored.PrivateData.Execution.RequestID)
			}
			if mode == "http400" || mode == "build_failure" {
				assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
				assert.Zero(t, stored.Quota)
				assert.Zero(t, sub.AmountUsed)
				assert.Zero(t, stored.PrivateData.SubscriptionAllowanceAllocated)
			} else {
				quota := 5000
				if mode == "zero_estimate" {
					quota = 1
				}
				assert.Equal(t, model.TaskStatusDispatching, stored.Status)
				assert.Equal(t, quota, stored.Quota)
				assert.Equal(t, int64(quota), sub.AmountUsed)
				assert.Equal(t, int64(quota), stored.PrivateData.SubscriptionAllowanceAllocated)
				require.NoError(t, db.Model(&sub).Updates(map[string]any{"allowance_epoch": 8, "amount_used": 100}).Error)
				_, err := model.PostConsumeUserSubscriptionDelta(sub.Id, -int64(quota), model.SubscriptionAllowanceRef{Epoch: stored.PrivateData.SubscriptionAllowanceEpoch, Allocated: stored.PrivateData.SubscriptionAllowanceAllocated})
				require.NoError(t, err)
				require.NoError(t, db.First(&sub, sub.Id).Error)
				assert.Equal(t, int64(100), sub.AmountUsed, "old-generation reconciliation must not credit new allowance")
			}
			account, err := model.GetUserById(user.Id, false)
			require.NoError(t, err)
			assert.Equal(t, 100000, account.Quota)
			assert.Zero(t, account.UsedQuota)
			if mode == "build_failure" {
				assert.Zero(t, calls.Load())
			} else {
				assert.Equal(t, int32(1), calls.Load())
			}
		})
	}
}
