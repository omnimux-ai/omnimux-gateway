package model

import (
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedSubscriptionSettlement(t *testing.T, wallet int, total, used int64) (User, UserSubscription) {
	t.Helper()
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() { require.NoError(t, DB.Where("1 = 1").Delete(&SubscriptionPreConsumeRecord{}).Error) })
	user := createReserveTestUser(t, wallet)
	plan := SubscriptionPlan{Title: t.Name(), QuotaResetPeriod: SubscriptionResetNever}
	require.NoError(t, DB.Create(&plan).Error)
	now := GetDBTimestamp()
	sub := UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: total, AmountUsed: used,
		Status: "active", StartTime: now - 60, EndTime: now + 3600}
	require.NoError(t, DB.Create(&sub).Error)
	return user, sub
}

// postDelta calls the strict supplemental-reservation primitive and discards the
// returned claim, for assertions that only care about the error.
func postDelta(subID int, delta int64, ref SubscriptionAllowanceRef) error {
	_, err := PostConsumeUserSubscriptionDelta(subID, delta, ref)
	return err
}

// allowanceClaim builds the claim a caller holds after charging `allocated` quota
// into the subscription's currently open allowance generation.
func allowanceClaim(t *testing.T, subID int, allocated int64) SubscriptionAllowanceRef {
	t.Helper()
	return SubscriptionAllowanceRef{Epoch: getSubscriptionResetSub(t, subID).AllowanceEpoch, Allocated: allocated}
}

func TestSubscriptionSettlementAllocation(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		wallet                int
		total, used, delta    int64
		subDelta, walletDelta int64
		wantUsed              int64
		wantWallet            int
	}{
		{"original overrun", 0, 1000, 950, 100, 50, 50, 1000, -50},
		{"positive wallet covers excess", 80, 1000, 950, 100, 50, 50, 1000, 30},
		{"normal consumption", 0, 1000, 950, 20, 20, 0, 970, 0},
		{"unlimited subscription", 0, 0, 950, 100, 100, 0, 1050, 0},
		{"ordinary reservation refund", 0, 1000, 950, -40, -40, 0, 910, 0},
		{"already admitted completion in debt", -50, 1000, 1000, 30, 0, 30, 1000, -80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, sub := seedSubscriptionSettlement(t, tc.wallet, tc.total, tc.used)
			result, err := SettleUserSubscriptionDelta(user.Id, sub.Id, tc.delta, 0, allowanceClaim(t, sub.Id, 1000))
			require.NoError(t, err)
			assert.Equal(t, tc.subDelta, result.SubscriptionDelta)
			assert.Equal(t, tc.walletDelta, result.WalletDelta)
			assert.Equal(t, tc.wantUsed, result.AmountUsed)
			assert.Equal(t, int(tc.delta), result.QuotaDelta)
			assert.Equal(t, tc.wantWallet, getUserQuotaFromDB(t, user.Id))
			assert.Equal(t, tc.wantUsed, getSubscriptionResetSub(t, sub.Id).AmountUsed)
		})
	}
}

func TestSubscriptionSettlementRefundUsesOnlyThisRequestWallet(t *testing.T) {
	user, sub := seedSubscriptionSettlement(t, 0, 1000, 950)
	first, err := SettleUserSubscriptionDelta(user.Id, sub.Id, 100, 0, allowanceClaim(t, sub.Id, 150))
	require.NoError(t, err)
	second, err := SettleUserSubscriptionDelta(user.Id, sub.Id, 80, 0, allowanceClaim(t, sub.Id, 0))
	require.NoError(t, err)
	assert.EqualValues(t, 50, first.WalletDelta)
	assert.EqualValues(t, 80, second.WalletDelta)
	assert.Equal(t, -130, getUserQuotaFromDB(t, user.Id))

	partial, err := SettleUserSubscriptionDelta(user.Id, sub.Id, -20, first.WalletDelta, allowanceClaim(t, sub.Id, 150))
	require.NoError(t, err)
	assert.EqualValues(t, -20, partial.WalletDelta)
	assert.Zero(t, partial.SubscriptionDelta)
	assert.Equal(t, -110, getUserQuotaFromDB(t, user.Id))

	remaining, err := SettleUserSubscriptionDelta(user.Id, sub.Id, -180, first.WalletDelta+partial.WalletDelta, allowanceClaim(t, sub.Id, 150))
	require.NoError(t, err)
	assert.EqualValues(t, -30, remaining.WalletDelta)
	assert.EqualValues(t, -150, remaining.SubscriptionDelta)
	assert.Equal(t, -80, getUserQuotaFromDB(t, user.Id), "the unrelated request retains its entire wallet charge")
	assert.EqualValues(t, 850, getSubscriptionResetSub(t, sub.Id).AmountUsed)
}

func TestTaskSubscriptionSettlementReloadReplayAndResetRefund(t *testing.T) {
	user, sub := seedSubscriptionSettlement(t, 0, 1000, 950)
	task := Task{TaskID: "subscription-settlement", UserId: user.Id, Quota: 100,
		PrivateData: TaskPrivateData{BillingSource: "subscription", SubscriptionId: sub.Id, Key: "preserved"}}
	require.NoError(t, task.Insert())
	stale := task
	result, err := SettleTaskSubscriptionQuota(&task, 200)
	require.NoError(t, err)
	assert.Equal(t, 100, result.QuotaDelta)
	assert.EqualValues(t, 50, task.PrivateData.SubscriptionWalletConsumed)
	assert.Equal(t, 200, task.Quota)

	replayed, err := SettleTaskSubscriptionQuota(&stale, 200)
	require.NoError(t, err)
	assert.Zero(t, replayed.QuotaDelta)
	assert.Equal(t, 200, stale.Quota)
	assert.EqualValues(t, 50, stale.PrivateData.SubscriptionWalletConsumed)
	assert.Equal(t, -50, getUserQuotaFromDB(t, user.Id))

	var loaded Task
	require.NoError(t, DB.First(&loaded, task.ID).Error)
	assert.Equal(t, "preserved", loaded.PrivateData.Key)
	partial, err := SettleTaskSubscriptionQuota(&loaded, 180)
	require.NoError(t, err)
	assert.Equal(t, -20, partial.QuotaDelta)
	assert.EqualValues(t, -20, partial.WalletDelta)
	assert.EqualValues(t, 30, loaded.PrivateData.SubscriptionWalletConsumed)

	_, err = AdminResetUserSubscriptionsByPlan(user.Id, sub.PlanId, false)
	require.NoError(t, err)
	assert.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
	require.NoError(t, DB.First(&loaded, task.ID).Error)
	refunded, err := SettleTaskSubscriptionQuota(&loaded, 0)
	require.NoError(t, err)
	assert.Equal(t, -180, refunded.QuotaDelta)
	assert.EqualValues(t, -30, refunded.WalletDelta)
	assert.Zero(t, getUserQuotaFromDB(t, user.Id))
	assert.Zero(t, getSubscriptionResetSub(t, sub.Id).AmountUsed)
	assert.Zero(t, loaded.PrivateData.SubscriptionWalletConsumed)
	require.NoError(t, DB.First(&loaded, task.ID).Error)
	assert.Zero(t, loaded.Quota)
	assert.Zero(t, loaded.PrivateData.SubscriptionWalletConsumed)
	refunded, err = SettleTaskSubscriptionQuota(&loaded, 0)
	require.NoError(t, err)
	assert.Zero(t, refunded.QuotaDelta)
	assert.Zero(t, getUserQuotaFromDB(t, user.Id))
}

func TestSubscriptionDebtAdmissionAndStrictReservations(t *testing.T) {
	user, sub := seedSubscriptionSettlement(t, 0, 1000, 850)
	reservation, err := PreConsumeUserSubscription("initial", user.Id, "", 0, 100)
	require.NoError(t, err, "a zero wallet remains valid for prepaid subscriptions")
	assert.EqualValues(t, 950, reservation.AmountUsedAfter)
	require.Error(t, postDelta(sub.Id, 51, SubscriptionAllowanceRef{}), "supplemental reservation must stay strict")
	_, err = SettleUserSubscriptionDelta(user.Id, sub.Id, 100, 0, allowanceClaim(t, sub.Id, 0))
	require.NoError(t, err)
	require.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
	_, err = AdminResetUserSubscriptionsByPlan(user.Id, sub.PlanId, false)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("after-reset", user.Id, "", 0, 1)
	assert.ErrorIs(t, err, ErrUserWalletDebt)
	assert.ErrorIs(t, postDelta(sub.Id, 1, SubscriptionAllowanceRef{}), ErrUserWalletDebt)
	_, err = PreConsumeUserSubscription("initial", user.Id, "", 0, 100)
	assert.ErrorIs(t, err, ErrUserWalletDebt, "retrying the old preconsume ID cannot bypass debt")
	require.NoError(t, RefundSubscriptionPreConsume("initial"), "rollback remains available during debt")
	require.NoError(t, RefundSubscriptionPreConsume("initial"))
	assert.Equal(t, -50, getUserQuotaFromDB(t, user.Id), "reservation rollback cannot credit an unrelated wallet allocation")
	var count int64
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", "after-reset").Count(&count).Error)
	assert.Zero(t, count)
}

func TestSubscriptionDebtPersistsThroughReplacementAndNativeCredit(t *testing.T) {
	user, sub := seedSubscriptionSettlement(t, -50, 1000, 0)
	_, err := AdminInvalidateUserSubscription(sub.Id)
	require.NoError(t, err)
	var plan SubscriptionPlan
	require.NoError(t, DB.First(&plan, sub.PlanId).Error)
	_, err = CreateUserSubscriptionFromPlanTx(DB, user.Id, &plan, "test")
	require.NoError(t, err)
	assert.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
	for _, credit := range []struct{ amount, want int }{{20, -30}, {30, 0}, {10, 10}} {
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error { return creditTopUpQuota(tx, user.Id, credit.amount, nil) }))
		assert.Equal(t, credit.want, getUserQuotaFromDB(t, user.Id))
		if credit.want < 0 {
			assert.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
		} else {
			require.NoError(t, CheckUserWalletDebt(user.Id))
		}
	}
}

func TestSubscriptionSettlementRollbackOnInvalidStateOrWriteFailure(t *testing.T) {
	for _, mode := range []string{"missing user", "wrong owner", "missing subscription", "wallet write", "subscription write", "task write", "unpersisted task"} {
		t.Run(mode, func(t *testing.T) {
			user, sub := seedSubscriptionSettlement(t, 0, 1000, 950)
			task := Task{UserId: user.Id, Quota: 100, PrivateData: TaskPrivateData{BillingSource: "subscription", SubscriptionId: sub.Id}}
			if mode != "unpersisted task" {
				require.NoError(t, task.Insert())
			}
			switch mode {
			case "missing user":
				require.NoError(t, DB.Delete(&user).Error)
			case "wrong owner":
				require.NoError(t, DB.Model(&sub).Update("user_id", user.Id+1).Error)
			case "missing subscription":
				require.NoError(t, DB.Delete(&sub).Error)
			case "wallet write", "subscription write", "task write":
				table := map[string]string{"wallet write": "users", "subscription write": "user_subscriptions", "task write": "tasks"}[mode]
				const callback = "test:subscription_settlement_write_failure"
				require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Table == table {
						tx.AddError(errors.New("injected settlement write failure"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, DB.Callback().Update().Remove(callback)) })
			}
			result, err := SettleTaskSubscriptionQuota(&task, 200)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Equal(t, 100, task.Quota)
			assert.Zero(t, task.PrivateData.SubscriptionWalletConsumed)
			if mode != "missing user" {
				assert.Zero(t, getUserQuotaFromDB(t, user.Id))
			}
			if mode != "missing subscription" {
				assert.EqualValues(t, 950, getSubscriptionResetSub(t, sub.Id).AmountUsed)
			}
			if task.ID > 0 {
				var persisted Task
				require.NoError(t, DB.First(&persisted, task.ID).Error)
				assert.Equal(t, 100, persisted.Quota)
				assert.Zero(t, persisted.PrivateData.SubscriptionWalletConsumed)
			}
		})
	}
}

func TestSubscriptionSettlementArithmeticBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		wallet                         int
		total, used, delta, allocation int64
	}{
		{"oversized charge", 0, 1000, 950, math.MaxInt64, 0},
		{"refund underflow", 0, 1000, 950, math.MinInt64, 0},
		{"negative allocation", 0, 1000, 950, -1, -1},
		{"oversized allocation", 0, 1000, 950, -1, math.MaxInt64},
		{"unlimited usage overflow", 0, 0, math.MaxInt64, 1, 0},
		{"wallet debt underflow", -common.MaxWalletQuota, 1000, 1000, 1, 0},
		{"wallet refund overflow", common.MaxWalletQuota, 1000, 1000, -1, 1},
		{"invalid used amount", 0, 1000, -1, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, sub := seedSubscriptionSettlement(t, tc.wallet, tc.total, tc.used)
			_, err := SettleUserSubscriptionDelta(user.Id, sub.Id, tc.delta, tc.allocation, allowanceClaim(t, sub.Id, math.MaxInt64))
			require.Error(t, err)
			assert.Equal(t, tc.wallet, getUserQuotaFromDB(t, user.Id))
			assert.Equal(t, tc.used, getSubscriptionResetSub(t, sub.Id).AmountUsed)
		})
	}
	t.Run("strict unlimited reservation cannot wrap", func(t *testing.T) {
		user, sub := seedSubscriptionSettlement(t, 0, 0, math.MaxInt64)
		_, err := PreConsumeUserSubscription("overflow", user.Id, "", 0, 1)
		require.Error(t, err)
		require.Error(t, postDelta(sub.Id, 1, SubscriptionAllowanceRef{}))
		assert.EqualValues(t, math.MaxInt64, getSubscriptionResetSub(t, sub.Id).AmountUsed)
		require.NoError(t, postDelta(sub.Id, math.MinInt64, allowanceClaim(t, sub.Id, math.MaxInt64)))
		assert.Zero(t, getSubscriptionResetSub(t, sub.Id).AmountUsed)
	})
}

func TestSubscriptionSettlementCacheAndBatchPersistence(t *testing.T) {
	user, sub := seedSubscriptionSettlement(t, 80, 1000, 1000)
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true
	require.NoError(t, populateUserCache(user))
	reserved, err := TryReserveUserQuota(user.Id, 30)
	require.NoError(t, err)
	require.True(t, reserved)
	result, err := SettleUserSubscriptionDelta(user.Id, sub.Id, 100, 0, SubscriptionAllowanceRef{})
	require.NoError(t, err)
	assert.EqualValues(t, 100, result.WalletDelta)
	assert.Equal(t, -20, getUserQuotaFromDB(t, user.Id), "settlement debt is durable before batch flush")
	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	assert.Equal(t, -50, cached.Quota, "synchronize only the settlement delta, preserving queued reservation")
	batchUpdate()
	assert.Equal(t, -50, getUserQuotaFromDB(t, user.Id), "flush must not debit settlement twice")
	_, err = SettleUserSubscriptionDelta(user.Id, sub.Id, -40, 100, SubscriptionAllowanceRef{})
	require.NoError(t, err)
	assert.Equal(t, -10, getUserQuotaFromDB(t, user.Id))
	cached, err = GetUserCache(user.Id)
	require.NoError(t, err)
	assert.Equal(t, -10, cached.Quota)
}

func TestSubscriptionDebtIgnoresStalePositiveCache(t *testing.T) {
	user, _ := seedSubscriptionSettlement(t, -50, 1000, 0)
	useUserCacheMiniRedis(t)
	stale := user
	stale.Quota = 500
	require.NoError(t, populateUserCache(stale))
	assert.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
	_, err := PreConsumeUserSubscription("stale-cache", user.Id, "", 0, 1)
	assert.ErrorIs(t, err, ErrUserWalletDebt)
}

func TestSubscriptionSettlementCacheFailureDoesNotRepeatCommittedDebit(t *testing.T) {
	user, sub := seedSubscriptionSettlement(t, 0, 1000, 950)
	server := useUserCacheMiniRedis(t)
	require.NoError(t, populateUserCache(user))
	task := Task{UserId: user.Id, Quota: 100, PrivateData: TaskPrivateData{BillingSource: "subscription", SubscriptionId: sub.Id}}
	require.NoError(t, task.Insert())
	server.SetError("cache unavailable")
	result, err := SettleTaskSubscriptionQuota(&task, 200)
	require.NoError(t, err, "cache failure must not claim the committed funding transaction failed")
	assert.Equal(t, 100, result.QuotaDelta)
	assert.Equal(t, -50, getUserQuotaFromDB(t, user.Id))
	assert.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
	replayed, err := SettleTaskSubscriptionQuota(&task, 200)
	require.NoError(t, err)
	assert.Zero(t, replayed.QuotaDelta)
	assert.Equal(t, -50, getUserQuotaFromDB(t, user.Id))
}

func TestTaskPrivateDataPreservesStandaloneWalletAllocation(t *testing.T) {
	private := TaskPrivateData{SubscriptionWalletConsumed: 50}
	value, err := private.Value()
	require.NoError(t, err)
	require.NotNil(t, value)
	var restored TaskPrivateData
	require.NoError(t, restored.Scan(value))
	assert.EqualValues(t, 50, restored.SubscriptionWalletConsumed)
}

func TestTaskSubscriptionSettlementConcurrentCompletionAndReplay(t *testing.T) {
	for _, replay := range []bool{false, true} {
		name := "independent admitted tasks"
		if replay {
			name = "same task replay"
		}
		t.Run(name, func(t *testing.T) {
			user, sub := seedSubscriptionSettlement(t, 0, 1000, 1000)
			first := Task{UserId: user.Id, Quota: 100, PrivateData: TaskPrivateData{BillingSource: "subscription", SubscriptionId: sub.Id}}
			require.NoError(t, first.Insert())
			second := first
			if !replay {
				second.ID = 0
				require.NoError(t, second.Insert())
			}
			start := make(chan struct{})
			var group sync.WaitGroup
			results := make([]*SubscriptionQuotaSettlement, 2)
			errors := make([]error, 2)
			for i, task := range []*Task{&first, &second} {
				group.Add(1)
				go func() {
					defer group.Done()
					<-start
					results[i], errors[i] = SettleTaskSubscriptionQuota(task, 150)
				}()
			}
			close(start)
			group.Wait()
			for _, err := range errors {
				require.NoError(t, err)
			}
			want := 100
			if replay {
				want = 50
			}
			assert.Equal(t, want, results[0].QuotaDelta+results[1].QuotaDelta)
			assert.Equal(t, -want, getUserQuotaFromDB(t, user.Id))
			assert.EqualValues(t, 1000, getSubscriptionResetSub(t, sub.Id).AmountUsed)
		})
	}
}

func TestSubscriptionSettlementCacheMissAndPendingDebt(t *testing.T) {
	t.Run("missing cache hydrates committed debt", func(t *testing.T) {
		user, sub := seedSubscriptionSettlement(t, 0, 1000, 950)
		useUserCacheMiniRedis(t)
		_, err := SettleUserSubscriptionDelta(user.Id, sub.Id, 100, 0, SubscriptionAllowanceRef{})
		require.NoError(t, err)
		cached, err := GetUserCache(user.Id)
		require.NoError(t, err)
		assert.Equal(t, -50, cached.Quota)
		assert.Equal(t, -50, getUserQuotaFromDB(t, user.Id))
	})
	t.Run("cached debt awaiting native batch persistence denies admission", func(t *testing.T) {
		user, _ := seedSubscriptionSettlement(t, 10, 1000, 0)
		useUserCacheMiniRedis(t)
		cached := user
		cached.Quota = -5
		require.NoError(t, populateUserCache(cached))
		assert.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
		_, err := PreConsumeUserSubscription("pending-wallet-debt", user.Id, "", 0, 1)
		assert.ErrorIs(t, err, ErrUserWalletDebt)
	})
}
