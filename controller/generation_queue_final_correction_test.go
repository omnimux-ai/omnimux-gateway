package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Regression from the final review's native worker + loopback HTTP probe.
func TestGenerationQueueImmediateFailureRefundDBError(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"id":"native-failed","status":"FAILURE"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	for i := 2; i <= 5; i++ {
		active := insertCorrectionTask(t, user, i, false)
		require.NoError(t, db.Model(active).Update("status", model.TaskStatusDispatching).Error)
	}
	insertCorrectionTask(t, user, 6, false)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("final_native_refund_error", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]interface{})
		if tx.Statement.Table == "users" && ok {
			if expr, ok := values["quota"].(clause.Expr); ok && strings.Contains(expr.SQL, "quota +") {
				tx.AddError(errors.New("native refund DB unavailable"))
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
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	next, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	t.Logf("POST=%d status=%s quota=%d wallet=%d next=%d reason=%q", calls.Load(), stored.Status, stored.Quota, account.Quota, len(next), stored.FailReason)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Equal(t, 5000, stored.Quota)
	assert.Empty(t, next)
	assert.Contains(t, stored.FailReason, "待核对")
	assert.Equal(t, 95000, account.Quota)
	assert.EqualValues(t, 1, calls.Load())
}

func TestGenerationQueueImmediateFailureNativeFundingMatrix(t *testing.T) {
	for _, funding := range []string{"wallet", "subscription"} {
		for _, mode := range []string{"success", "refund_error", "persist_error", "late_owner"} {
			t.Run(funding+"/"+mode, func(t *testing.T) {
				var calls, refundAttempts atomic.Int32
				db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					_, _ = w.Write([]byte(`{"id":"native-failed","status":"FAILURE"}`))
				}))
				var sub model.UserSubscription
				if funding == "subscription" {
					require.NoError(t, db.Model(&user).Update("setting", `{"billing_preference":"subscription_only"}`).Error)
					plan := model.SubscriptionPlan{Title: "test native plan", TotalAmount: 100000}
					require.NoError(t, db.Create(&plan).Error)
					sub = model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: "active", StartTime: common.GetTimestamp() - 1, EndTime: common.GetTimestamp() + 3600, AllowanceEpoch: 7}
					require.NoError(t, db.Create(&sub).Error)
				}
				task := insertCorrectionTask(t, user, 1, false)
				for i := 2; i <= 5; i++ {
					active := insertCorrectionTask(t, user, i, false)
					require.NoError(t, db.Model(active).Update("status", model.TaskStatusDispatching).Error)
				}
				sixth := insertCorrectionTask(t, user, 6, false)
				isRefund := func(tx *gorm.DB) bool {
					if funding == "subscription" {
						values, ok := tx.Statement.Dest.(map[string]interface{})
						return tx.Statement.Table == "user_subscriptions" && ok && values["amount_used"] == int64(0)
					}
					values, ok := tx.Statement.Dest.(map[string]interface{})
					if tx.Statement.Table != "users" || !ok {
						return false
					}
					expr, ok := values["quota"].(clause.Expr)
					return ok && strings.Contains(expr.SQL, "quota +")
				}
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("final_funding_release", func(tx *gorm.DB) {
					if isRefund(tx) {
						refundAttempts.Add(1)
						if mode == "refund_error" {
							tx.AddError(errors.New("native funding release DB unavailable"))
						}
					}
					if result, ok := tx.Statement.Dest.(*model.Task); ok && result.Status == model.TaskStatusFailure {
						assert.EqualValues(t, 1, refundAttempts.Load(), "terminal publication must follow one synchronous native release")
						if mode == "persist_error" {
							tx.AddError(errors.New("terminal row unavailable"))
						}
					}
				}))
				if mode == "late_owner" {
					require.NoError(t, db.Callback().Update().After("gorm:update").Register("final_replace_owner", func(tx *gorm.DB) {
						if !isRefund(tx) || tx.Error != nil {
							return
						}
						conn := tx.Session(&gorm.Session{NewDB: true})
						var row model.Task
						if err := conn.First(&row, task.ID).Error; err != nil {
							tx.AddError(err)
							return
						}
						row.PrivateData.GenerationQueue.ClaimOwner = "new-owner"
						row.FailReason = "new-owner state"
						tx.AddError(conn.Model(&row).Select("private_data", "fail_reason").Updates(&row).Error)
					}))
				}
				claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
				stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
				require.NoError(t, err)
				require.True(t, found)
				assert.Equal(t, task.ID, stored.ID)
				assert.Equal(t, task.Properties.Input, stored.Properties.Input)
				assert.Equal(t, funding, stored.PrivateData.BillingSource)
				require.NotNil(t, stored.PrivateData.BillingContext)
				assert.EqualValues(t, 1, refundAttempts.Load())
				account, err := model.GetUserById(user.Id, false)
				require.NoError(t, err)
				assert.Zero(t, account.UsedQuota)
				var channel model.Channel
				require.NoError(t, db.First(&channel).Error)
				assert.Zero(t, channel.UsedQuota)
				next, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
				require.NoError(t, err)
				if mode == "success" {
					assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
					assert.Zero(t, stored.Quota)
					assert.Zero(t, stored.PrivateData.SubscriptionAllowanceAllocated)
					assert.Zero(t, stored.PrivateData.SubscriptionWalletConsumed)
					require.Len(t, next, 1)
					assert.Equal(t, sixth.ID, next[0].ID)
				} else {
					assert.Equal(t, model.TaskStatusDispatching, stored.Status)
					assert.Equal(t, 5000, stored.Quota)
					assert.Empty(t, next)
					if mode == "late_owner" {
						require.NotNil(t, stored.PrivateData.GenerationQueue)
						assert.Equal(t, "new-owner", stored.PrivateData.GenerationQueue.ClaimOwner)
						assert.Equal(t, "new-owner state", stored.FailReason)
					} else {
						assert.Contains(t, stored.FailReason, "待核对")
					}
				}
				if funding == "subscription" {
					require.NoError(t, db.First(&sub, sub.Id).Error)
					assert.Equal(t, 100000, account.Quota)
					if mode == "refund_error" {
						assert.EqualValues(t, 5000, sub.AmountUsed)
						assert.EqualValues(t, 5000, stored.PrivateData.SubscriptionAllowanceAllocated)
					} else {
						assert.Zero(t, sub.AmountUsed)
					}
				} else if mode == "refund_error" {
					assert.Equal(t, 95000, account.Quota)
				} else {
					assert.Equal(t, 100000, account.Quota)
				}
				if mode != "late_owner" {
					require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "correction"))
				}
				dispatchInfluencerGeneration(context.Background(), stored, scanner, "correction")
				assert.EqualValues(t, 1, calls.Load(), "result reload must never send a second POST")
				assert.EqualValues(t, 1, refundAttempts.Load(), "result reload must never repeat a refund")
				t.Logf("funding=%s mode=%s status=%s quota=%d allocated=%d wallet=%d refund_attempts=%d POST=%d", funding, mode, stored.Status, stored.Quota, stored.PrivateData.SubscriptionAllowanceAllocated, account.Quota, refundAttempts.Load(), calls.Load())
			})
		}
	}
}

func TestGenerationQueueRestartScannerReconcilesWithoutSecondPOST(t *testing.T) {
	var calls atomic.Int32
	db, user, oldScanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"unexpected":"accepted but receipt lost"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, oldScanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], oldScanner, "correction")
	require.NoError(t, db.Model(task).Update("fail_reason", "").Error)
	for i := 2; i <= 5; i++ {
		preparing := insertCorrectionTask(t, user, i, false)
		preparing.Status = model.TaskStatusDispatching
		preparing.PrivateData.GenerationQueue.ClaimOwner = oldScanner.TaskID
		require.NoError(t, preparing.Update())
	}
	sixth := insertCorrectionTask(t, user, 6, false)
	require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("task_id = ?", oldScanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
	require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
	scanner, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
	require.NoError(t, err)
	scanner, acquired, err := model.ClaimSystemTask(scanner.ID, scanner.Type, "restart", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { generationQueueHandler{}.Run(ctx, scanner, "restart"); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("restart scanner not collected")
		}
	}()
	require.Eventually(t, func() bool {
		stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
		return err == nil && found && strings.Contains(stored.FailReason, "待核对")
	}, 5*time.Second, time.Millisecond, "restart scan must mark interrupted submission for reconciliation")
	var rows []model.Task
	require.NoError(t, db.Where("user_id = ? AND status = ?", user.Id, model.TaskStatusDispatching).Find(&rows).Error)
	require.Len(t, rows, 5)
	for _, row := range rows {
		assert.Contains(t, row.FailReason, "待核对")
		if row.ID == task.ID {
			assert.Equal(t, 5000, row.Quota)
			assert.Equal(t, "wallet", row.PrivateData.BillingSource)
		}
	}
	var waiting model.Task
	require.NoError(t, db.First(&waiting, sixth.ID).Error)
	assert.Equal(t, model.TaskStatusWaiting, waiting.Status)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 95000, account.Quota)
	assert.EqualValues(t, 1, calls.Load())
}

func TestGenerationQueueBlockedHTTPPublicReasonIsEmpty(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	_, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte(`{"id":"native-success","status":"SUCCESS"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	go func() {
		dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
		close(done)
	}()
	defer func() {
		close(release)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("native worker not collected")
		}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "local provider never received POST")
	}
	require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "correction"))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/task/self?task_id="+task.TaskID, nil)
	c.Set("id", user.Id)
	GetUserTask(c)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Items []struct {
				TaskID     string           `json:"task_id"`
				Status     model.TaskStatus `json:"status"`
				FailReason string           `json:"fail_reason"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, task.TaskID, response.Data.Items[0].TaskID)
	assert.Equal(t, model.TaskStatusDispatching, response.Data.Items[0].Status)
	assert.Empty(t, response.Data.Items[0].FailReason)
	for _, private := range []string{"private_data", "billing_context", "claim_owner", "send_started_at", "test-only"} {
		assert.NotContains(t, recorder.Body.String(), private)
	}
}

// Unconfirmed dispatches expire inside the long-lived scan loop: funded rows
// release their reservation once without rolling back usage, unfunded rows only
// free their slot, and the provider is never contacted again.
func TestGenerationQueueScannerExpiresUnconfirmedDispatches(t *testing.T) {
	previousTimeout := constant.GenerationDispatchTimeoutMinutes
	constant.GenerationDispatchTimeoutMinutes = 60
	t.Cleanup(func() { constant.GenerationDispatchTimeoutMinutes = previousTimeout })
	var calls atomic.Int32
	db, user, oldScanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"unexpected":"accepted but receipt lost"}`))
	}))
	sent := insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, oldScanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], oldScanner, "correction")
	stale := common.GetTimestamp() - 61*60
	require.NoError(t, db.Model(&model.Task{}).Where("id = ?", sent.ID).Update("start_time", stale).Error)
	var preparingIDs []int64
	for i := 2; i <= 5; i++ {
		preparing := insertCorrectionTask(t, user, i, false)
		preparing.Status = model.TaskStatusDispatching
		preparing.StartTime = stale
		preparing.PrivateData.GenerationQueue.ClaimOwner = oldScanner.TaskID
		require.NoError(t, preparing.Update())
		preparingIDs = append(preparingIDs, preparing.ID)
	}
	sixth := insertCorrectionTask(t, user, 6, false)
	before, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	require.Equal(t, 95000, before.Quota)

	require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("task_id = ?", oldScanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
	require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
	scanner, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
	require.NoError(t, err)
	scanner, acquired, err := model.ClaimSystemTask(scanner.ID, scanner.Type, "expiry", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { generationQueueHandler{}.Run(ctx, scanner, "expiry"); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("expiry scanner not collected")
		}
	}()

	// The shared provider also loses the sixth task's receipt, so it settles in
	// DISPATCHING holding its own 5000 reservation; that makes the wallet exact.
	require.Eventually(t, func() bool {
		var sixthRow model.Task
		return db.First(&sixthRow, sixth.ID).Error == nil && sixthRow.Status == model.TaskStatusDispatching && sixthRow.FailReason != ""
	}, 5*time.Second, 10*time.Millisecond, "expired dispatches must free slots for queued work")
	var expired model.Task
	require.NoError(t, db.First(&expired, sent.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), expired.Status)
	assert.Zero(t, expired.Quota)
	assert.Contains(t, expired.FailReason, "60")
	for _, id := range preparingIDs {
		var unfunded model.Task
		require.NoError(t, db.First(&unfunded, id).Error)
		assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), unfunded.Status)
		assert.Contains(t, unfunded.FailReason, "人工核对")
	}
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	// 95000 + 5000 returned for the expired row - 5000 reserved by the sixth.
	assert.Equal(t, 95000, account.Quota)
	assert.Equal(t, before.UsedQuota, account.UsedQuota)
	assert.Equal(t, before.RequestCount, account.RequestCount)
	var refunds int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeRefund).Count(&refunds).Error)
	assert.EqualValues(t, 1, refunds)
	assert.EqualValues(t, 2, calls.Load(), "the expired row is never resent; only the newly claimed row is sent")
}

func TestGenerationQueueExpiryDisabledKeepsReconciliation(t *testing.T) {
	previousTimeout := constant.GenerationDispatchTimeoutMinutes
	constant.GenerationDispatchTimeoutMinutes = 0
	t.Cleanup(func() { constant.GenerationDispatchTimeoutMinutes = previousTimeout })
	db, user, oldScanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"unexpected":"accepted but receipt lost"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, oldScanner.TaskID)
	require.NoError(t, err)
	dispatchInfluencerGeneration(context.Background(), claimed[0], oldScanner, "correction")
	require.NoError(t, db.Model(&model.Task{}).Where("id = ?", task.ID).Update("start_time", common.GetTimestamp()-24*3600).Error)
	insertCorrectionTask(t, user, 2, false)
	require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("task_id = ?", oldScanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
	require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
	scanner, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
	require.NoError(t, err)
	scanner, acquired, err := model.ClaimSystemTask(scanner.ID, scanner.Type, "disabled", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	generationQueueHandler{}.Run(ctx, scanner, "disabled")
	var kept model.Task
	require.NoError(t, db.First(&kept, task.ID).Error)
	assert.Equal(t, model.TaskStatusDispatching, kept.Status)
	assert.Equal(t, 5000, kept.Quota)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Less(t, account.Quota, 100000)
}

// A submission that answers after its dispatch expired cannot overwrite the
// terminal row, charge again, or release the reservation a second time.
func TestGenerationQueueExpiredDispatchLateResponseKeepsTerminal(t *testing.T) {
	for _, tc := range []struct {
		name    string
		respond func(http.ResponseWriter)
	}{
		{"success", func(w http.ResponseWriter) { _, _ = w.Write([]byte(`{"id":"late-success","status":"SUCCESS"}`)) }},
		{"http400", func(w http.ResponseWriter) { w.WriteHeader(http.StatusBadRequest) }},
		{"immediate failure", func(w http.ResponseWriter) { _, _ = w.Write([]byte(`{"id":"late-failure","status":"FAILURE"}`)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				<-release
				tc.respond(w)
			}))
			task := insertCorrectionTask(t, user, 1, false)
			claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			go func() {
				dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
				close(done)
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				require.FailNow(t, "local provider never received POST")
			}
			// A newer scan owns the lease now; the in-flight row belongs to an old owner.
			require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
			require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
			newer, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
			require.NoError(t, err)
			newer, acquired, err := model.ClaimSystemTask(newer.ID, newer.Type, "newer", common.GetTimestamp()+60)
			require.NoError(t, err)
			require.True(t, acquired)
			require.NoError(t, db.Model(&model.Task{}).Where("id = ?", task.ID).Update("start_time", common.GetTimestamp()-61*60).Error)
			refundable, err := model.ExpireGenerationDispatches(context.Background(), newer, "newer", common.GetTimestamp()-60*60, 60, nil)
			require.NoError(t, err)
			require.Len(t, refundable, 1)
			require.True(t, service.ReleaseUnloggedTaskReservation(context.Background(), refundable[0], refundable[0].FailReason))

			close(release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				require.FailNow(t, "late worker not collected")
			}
			var stored model.Task
			require.NoError(t, db.First(&stored, task.ID).Error)
			assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
			assert.Zero(t, stored.Quota)
			assert.Empty(t, stored.PrivateData.UpstreamTaskID)
			account, err := model.GetUserById(user.Id, false)
			require.NoError(t, err)
			assert.Equal(t, 100000, account.Quota, "the late worker neither recharges nor refunds a second time")
			var refunds int64
			require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeRefund).Count(&refunds).Error)
			assert.EqualValues(t, 1, refunds)
		})
	}
}

// A worker that released its reservation but could not write the terminal row
// leaves the release marker, so expiry later frees the slot without refunding.
func TestGenerationQueueReleasedButUnpersistedRejectionIsNotRefundedAgain(t *testing.T) {
	previousTimeout := constant.GenerationDispatchTimeoutMinutes
	constant.GenerationDispatchTimeoutMinutes = 60
	t.Cleanup(func() { constant.GenerationDispatchTimeoutMinutes = previousTimeout })
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	task := insertCorrectionTask(t, user, 1, false)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("refuse_rejection_row", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]interface{})
		if ok && tx.Statement.Table == "tasks" && values["status"] == model.TaskStatusFailure && values["quota"] == 0 {
			tx.AddError(errors.New("rejection row write refused"))
		}
	}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	var pending model.Task
	require.NoError(t, db.First(&pending, task.ID).Error)
	require.Equal(t, model.TaskStatusDispatching, pending.Status)
	require.Positive(t, pending.PrivateData.GenerationQueue.ReleaseStartedAt)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	require.Equal(t, 100000, account.Quota, "the worker released the reservation in memory")

	require.NoError(t, db.Model(&model.Task{}).Where("id = ?", task.ID).Update("start_time", common.GetTimestamp()-61*60).Error)
	require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
	require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
	newer, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
	require.NoError(t, err)
	newer, acquired, err := model.ClaimSystemTask(newer.ID, newer.Type, "newer", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, expireGenerationDispatches(context.Background(), newer, "newer", nil))

	var expired model.Task
	require.NoError(t, db.First(&expired, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), expired.Status)
	assert.Contains(t, expired.FailReason, "人工核对")
	account, err = model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota, "expiry never refunds a reservation the worker already released")
	var refunds int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeRefund).Count(&refunds).Error)
	assert.Zero(t, refunds)
}

// Rows this scanner left unconfirmed still expire while the scan stays alive
// for other waiting work; only a running worker protects its row.
func TestGenerationQueueOwnFinishedDispatchesExpireWhileScanLives(t *testing.T) {
	previousTimeout := constant.GenerationDispatchTimeoutMinutes
	constant.GenerationDispatchTimeoutMinutes = 60
	t.Cleanup(func() { constant.GenerationDispatchTimeoutMinutes = previousTimeout })
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"unexpected":"accepted but receipt lost"}`))
	}))
	for i := 1; i <= 6; i++ {
		insertCorrectionTask(t, user, i, false)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { generationQueueHandler{}.Run(ctx, scanner, "correction"); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("scanner not collected")
		}
	}()
	require.Eventually(t, func() bool {
		var reviewed int64
		return db.Model(&model.Task{}).Where("user_id = ? AND status = ? AND fail_reason <> ''", user.Id, model.TaskStatusDispatching).Count(&reviewed).Error == nil && reviewed == 5
	}, 5*time.Second, 10*time.Millisecond, "five unconfirmed rows hold every slot")
	require.EqualValues(t, 5, calls.Load())
	require.NoError(t, db.Model(&model.Task{}).Where("user_id = ? AND status = ?", user.Id, model.TaskStatusDispatching).Update("start_time", common.GetTimestamp()-2*3600).Error)

	require.Eventually(t, func() bool {
		var failed int64
		return db.Model(&model.Task{}).Where("user_id = ? AND status = ?", user.Id, model.TaskStatusFailure).Count(&failed).Error == nil && failed == 5
	}, 20*time.Second, 50*time.Millisecond, "the scanner's own finished rows expire within one interval")
	require.Eventually(t, func() bool { return calls.Load() == 6 }, 5*time.Second, 10*time.Millisecond, "the sixth task is claimed and sent")
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	// 100000 − 5×5000 reserved + 5×5000 returned − 5000 held by the sixth.
	assert.Equal(t, 95000, account.Quota)
	assert.EqualValues(t, 6, calls.Load(), "expired rows are never resent")
}

// The immediate-failure path releases its reservation before publishing the
// terminal row; when that publication fails, expiry must not refund again.
func TestGenerationQueueReleasedButUnpersistedImmediateFailureIsNotRefundedAgain(t *testing.T) {
	previousTimeout := constant.GenerationDispatchTimeoutMinutes
	constant.GenerationDispatchTimeoutMinutes = 60
	t.Cleanup(func() { constant.GenerationDispatchTimeoutMinutes = previousTimeout })
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"native-failed","status":"FAILURE"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("refuse_immediate_failure_row", func(tx *gorm.DB) {
		if result, ok := tx.Statement.Dest.(*model.Task); ok && result.Status == model.TaskStatusFailure {
			tx.AddError(errors.New("terminal row unavailable"))
		}
	}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	var pending model.Task
	require.NoError(t, db.First(&pending, task.ID).Error)
	require.Equal(t, model.TaskStatusDispatching, pending.Status)
	require.Positive(t, pending.PrivateData.GenerationQueue.ReleaseStartedAt)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	require.Equal(t, 100000, account.Quota, "the worker released the reservation in memory")
	require.NoError(t, db.Callback().Update().Remove("refuse_immediate_failure_row"))

	require.NoError(t, db.Model(&model.Task{}).Where("id = ?", task.ID).Update("start_time", common.GetTimestamp()-61*60).Error)
	require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
	require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
	newer, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
	require.NoError(t, err)
	newer, acquired, err := model.ClaimSystemTask(newer.ID, newer.Type, "newer", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, expireGenerationDispatches(context.Background(), newer, "newer", nil))

	var expired model.Task
	require.NoError(t, db.First(&expired, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), expired.Status)
	assert.Contains(t, expired.FailReason, "人工核对")
	account, err = model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota, "expiry never refunds a reservation the worker already released")
	var refunds int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeRefund).Count(&refunds).Error)
	assert.Zero(t, refunds)
}
