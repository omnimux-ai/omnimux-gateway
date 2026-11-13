package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// fork: completed usage may create wallet debt; debt blocks new generation only.
var ErrUserWalletDebt = errors.New("user wallet has outstanding debt")

// CheckUserWalletDebt always checks durable quota before considering the cache.
// A stale positive cache must not forgive debt; a negative cache may include
// native wallet consumption still waiting in the batch queue.
func CheckUserWalletDebt(userID int) error {
	if userID <= 0 {
		return errors.New("invalid userId")
	}
	var user User
	if err := DB.Select("id", "quota").First(&user, userID).Error; err != nil {
		return err
	}
	return checkSubscriptionWalletDebt(&user)
}

func checkSubscriptionWalletDebt(user *User) error {
	if user.Quota < 0 {
		return ErrUserWalletDebt
	}
	if common.RedisEnabled {
		if cached, err := cacheGetUserBase(user.Id); err == nil && cached.Quota < 0 {
			return ErrUserWalletDebt
		}
	}
	return nil
}

// SubscriptionQuotaSettlement describes signed consumption: positive consumes,
// negative refunds. SubscriptionDelta retains native cross-reset refund
// semantics (usage clamps to zero); AmountUsed is the resulting durable usage.
// Allowance is the caller's claim after the settlement, carrying the allowance
// generation the applied subscription delta belongs to.
type SubscriptionQuotaSettlement struct {
	SubscriptionDelta int64
	WalletDelta       int64
	AmountUsed        int64
	QuotaDelta        int
	Allowance         SubscriptionAllowanceRef
}

// SettleUserSubscriptionDelta atomically funds already-completed usage from the
// remaining subscription allowance, then the native wallet. walletConsumed is
// this request's prior wallet allocation, never the user's total debt. ref is the
// caller's claim on the allowance generation that funded this request; the
// returned settlement reports the claim's next state.
func SettleUserSubscriptionDelta(userID, subscriptionID int, delta, walletConsumed int64, ref SubscriptionAllowanceRef) (*SubscriptionQuotaSettlement, error) {
	var result *SubscriptionQuotaSettlement
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = settleUserSubscriptionDeltaTx(tx, userID, subscriptionID, delta, walletConsumed, ref)
		return err
	})
	if err != nil {
		return nil, err
	}
	syncSubscriptionSettlementCache(userID, result.WalletDelta)
	return result, nil
}

func settleUserSubscriptionDeltaTx(tx *gorm.DB, userID, subscriptionID int, delta, walletConsumed int64, ref SubscriptionAllowanceRef) (*SubscriptionQuotaSettlement, error) {
	if userID <= 0 || subscriptionID <= 0 {
		return nil, errors.New("invalid userId or subscriptionId")
	}
	if delta < -common.MaxQuota || delta > common.MaxQuota || walletConsumed < 0 || walletConsumed > common.MaxQuota {
		return nil, errors.New("subscription settlement quota out of range")
	}
	var user User
	if err := lockForUpdate(tx).Select("id", "quota").First(&user, userID).Error; err != nil {
		return nil, err
	}
	var sub UserSubscription
	if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionID, userID).First(&sub).Error; err != nil {
		return nil, err
	}
	if sub.AmountUsed < 0 {
		return nil, errors.New("invalid subscription used amount")
	}
	result := &SubscriptionQuotaSettlement{SubscriptionDelta: delta, QuotaDelta: int(delta), Allowance: ref}
	if delta > 0 && sub.AmountTotal > 0 {
		remaining := max(sub.AmountTotal-sub.AmountUsed, 0)
		result.SubscriptionDelta = min(delta, remaining)
		result.WalletDelta = delta - result.SubscriptionDelta
	} else if delta < 0 {
		result.WalletDelta = -min(-delta, walletConsumed)
		result.SubscriptionDelta = delta - result.WalletDelta
	}
	if walletConsumed+result.WalletDelta > common.MaxQuota {
		return nil, errors.New("subscription wallet allocation exceeds quota limit")
	}
	// fork: only the claim's own current-generation allocation may be given back.
	// The wallet part above stays untouched: it refunds this request's real wallet
	// charge and is independent of which allowance generation funded the rest.
	if result.SubscriptionDelta < 0 {
		restore := int64(0)
		if ref.IsCurrent(sub.AllowanceEpoch) && ref.Allocated > 0 {
			restore = ref.Allocated
			if result.SubscriptionDelta > -restore {
				restore = -result.SubscriptionDelta
			}
		}
		result.SubscriptionDelta = -restore
		result.Allowance.Allocated -= restore
	} else if result.SubscriptionDelta > 0 {
		// fork: the positive late-settlement policy is unchanged; a charge landing in a
		// newer generation re-tags the claim so a later refund restores exactly this.
		if ref.IsCurrent(sub.AllowanceEpoch) {
			result.Allowance.Allocated += result.SubscriptionDelta
		} else {
			result.Allowance = SubscriptionAllowanceRef{Epoch: sub.AllowanceEpoch, Allocated: result.SubscriptionDelta}
		}
	}
	if result.SubscriptionDelta > 0 && sub.AmountUsed > math.MaxInt64-result.SubscriptionDelta {
		return nil, errors.New("subscription used amount overflow")
	}
	result.AmountUsed = max(sub.AmountUsed+result.SubscriptionDelta, 0)
	// Native wallets use a JavaScript-safe signed domain. Check before subtracting
	// so even corrupt extreme stored values fail without wrapping or partial writes.
	if result.WalletDelta != 0 {
		quota := int64(user.Quota)
		if quota < -common.MaxWalletQuota || quota > common.MaxWalletQuota ||
			(result.WalletDelta > 0 && quota < -common.MaxWalletQuota+result.WalletDelta) ||
			(result.WalletDelta < 0 && quota > common.MaxWalletQuota+result.WalletDelta) {
			return nil, errors.New("subscription settlement wallet quota out of range")
		}
		res := tx.Model(&User{}).Where("id = ?", userID).
			Update("quota", gorm.Expr("quota - ?", result.WalletDelta))
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected != 1 {
			return nil, gorm.ErrRecordNotFound
		}
	}
	if result.AmountUsed != sub.AmountUsed {
		res := tx.Model(&UserSubscription{}).Where("id = ?", sub.Id).
			Updates(map[string]any{"amount_used": result.AmountUsed, "updated_at": common.GetTimestamp()})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected != 1 {
			return nil, gorm.ErrRecordNotFound
		}
	}
	return result, nil
}

// SettleTaskSubscriptionQuota reloads and locks the durable task. Funding,
// request-specific wallet attribution and the charged quota commit together,
// making a stale retry of the same final quota a no-op.
func SettleTaskSubscriptionQuota(task *Task, actualQuota int) (*SubscriptionQuotaSettlement, error) {
	if task == nil || task.ID <= 0 || actualQuota < 0 || actualQuota > common.MaxQuota {
		return nil, errors.New("invalid task subscription settlement")
	}
	var stored Task
	var result *SubscriptionQuotaSettlement
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&stored, task.ID).Error; err != nil {
			return err
		}
		if stored.UserId != task.UserId || stored.PrivateData.BillingSource != "subscription" ||
			stored.PrivateData.SubscriptionId <= 0 || stored.Quota < 0 || stored.Quota > common.MaxQuota ||
			stored.PrivateData.SubscriptionWalletConsumed > int64(stored.Quota) {
			return errors.New("invalid task subscription funding state")
		}
		var err error
		result, err = settleUserSubscriptionDeltaTx(tx, stored.UserId, stored.PrivateData.SubscriptionId,
			int64(actualQuota)-int64(stored.Quota), stored.PrivateData.SubscriptionWalletConsumed,
			SubscriptionAllowanceRef{
				Epoch:     stored.PrivateData.SubscriptionAllowanceEpoch,
				Allocated: stored.PrivateData.SubscriptionAllowanceAllocated,
			})
		if err != nil {
			return err
		}
		stored.Quota = actualQuota
		stored.PrivateData.SubscriptionWalletConsumed += result.WalletDelta
		// fork: the task carries the allowance generation it was charged in, so a
		// settlement that lands after a reset cannot replenish the new generation.
		stored.PrivateData.SubscriptionAllowanceEpoch = result.Allowance.Epoch
		stored.PrivateData.SubscriptionAllowanceAllocated = result.Allowance.Allocated
		if result.QuotaDelta == 0 {
			return nil
		}
		res := tx.Model(&Task{}).Where("id = ?", stored.ID).Updates(map[string]any{
			"quota": actualQuota, "private_data": stored.PrivateData,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	task.Quota = stored.Quota
	task.PrivateData.SubscriptionWalletConsumed = stored.PrivateData.SubscriptionWalletConsumed
	task.PrivateData.SubscriptionAllowanceEpoch = stored.PrivateData.SubscriptionAllowanceEpoch
	task.PrivateData.SubscriptionAllowanceAllocated = stored.PrivateData.SubscriptionAllowanceAllocated
	syncSubscriptionSettlementCache(stored.UserId, result.WalletDelta)
	return result, nil
}

func syncSubscriptionSettlementCache(userID int, walletDelta int64) {
	if walletDelta == 0 {
		return
	}
	if err := cacheIncrUserQuota(userID, -walletDelta); err != nil {
		common.SysError(fmt.Sprintf("subscription settlement committed but wallet cache sync failed: user=%d error=%s", userID, err))
		if invalidateErr := invalidateUserCache(userID); invalidateErr != nil {
			common.SysError(fmt.Sprintf("subscription wallet cache invalidation failed: user=%d error=%s", userID, invalidateErr))
		}
	}
}
