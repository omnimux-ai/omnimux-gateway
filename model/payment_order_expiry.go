package model

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const PaymentOrderExpiryMinAge = 25 * time.Hour

// PaymentOrderExpiryCandidate is a snapshot of the local order identity before
// querying the payment provider. It is never migrated or written as a model.
type PaymentOrderExpiryCandidate struct {
	ID              int
	UserID          int
	TradeNo         string
	PaymentProvider string
	CreateTime      int64
	Status          string
}

func agedPendingPaymentOrders(db *gorm.DB, subscription bool, cutoff int64) *gorm.DB {
	var order any = &TopUp{}
	if subscription {
		order = &SubscriptionOrder{}
	}
	return db.Model(order).
		Where("status = ? AND create_time > 0 AND create_time <= ? AND trade_no <> ''", common.TopUpStatusPending, cutoff).
		Where("payment_provider IN ?", []string{PaymentProviderWaffo, PaymentProviderWaffoPancake})
}

func ListPaymentOrderExpiryCandidates(ctx context.Context, subscription bool, afterID int, cutoff int64, limit int) ([]PaymentOrderExpiryCandidate, error) {
	cutoff = min(cutoff, common.GetTimestamp()-int64(PaymentOrderExpiryMinAge/time.Second))
	var orders []PaymentOrderExpiryCandidate
	err := agedPendingPaymentOrders(DB.WithContext(ctx), subscription, cutoff).
		Select("id", "user_id", "trade_no", "payment_provider", "create_time", "status").
		Where("id > ?", afterID).Order("id ASC").Limit(limit).Find(&orders).Error
	return orders, err
}

// ExpireConfirmedUnpaidOrder applies the provider's terminal result only while
// the worker still owns the lease and the checked order is still pending.
func ExpireConfirmedUnpaidOrder(ctx context.Context, taskID, runnerID string, subscription bool, candidate PaymentOrderExpiryCandidate, cutoff int64) (bool, error) {
	var expired bool
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lease SystemTaskLock
		activeTask := tx.Model(&SystemTask{}).Select("1").
			Where("type = ? AND task_id = ? AND locked_by = ? AND status = ?", SystemTaskTypePaymentOrderExpiry, taskID, runnerID, SystemTaskStatusRunning)
		err := lockForUpdate(tx).Where("type = ? AND task_id = ? AND locked_by = ? AND locked_until >= ?",
			SystemTaskTypePaymentOrderExpiry, taskID, runnerID, common.GetTimestamp()).Where("EXISTS (?)", activeTask).Take(&lease).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSystemTaskLockLost
		}
		if err != nil {
			return err
		}

		cutoff := min(cutoff, common.GetTimestamp()-int64(PaymentOrderExpiryMinAge/time.Second))
		orderQuery := agedPendingPaymentOrders(tx, subscription, cutoff).Where("id = ?", candidate.ID)
		var current PaymentOrderExpiryCandidate
		err = lockForUpdate(orderQuery.Session(&gorm.Session{})).
			Select("id", "user_id", "trade_no", "payment_provider", "create_time", "status").Take(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := common.GetTimestamp()
		if lease.LockedUntil < now {
			return ErrSystemTaskLockLost
		}
		if err != nil || current != candidate || current.Status != common.TopUpStatusPending {
			return nil
		}
		// MySQL's collation can match differently cased values in the SQL filter.
		if current.PaymentProvider != PaymentProviderWaffo && current.PaymentProvider != PaymentProviderWaffoPancake {
			return nil
		}
		result := orderQuery.Updates(map[string]any{"status": common.TopUpStatusExpired, "complete_time": now})
		expired = result.RowsAffected == 1
		return result.Error
	})
	return expired && err == nil, err
}
