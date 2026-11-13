package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type HostedPurchaseResult struct {
	Order       *HostedOrder
	Entitlement *HostedEntitlement
	SubOrderID  int
	AmountUSD   float64
	Quota       int
	Idempotent  bool
}

func validateHostedQty(qty int) error {
	if qty < 1 || qty > HostedMaxOpenQuantity {
		return ErrHostedInvalidQuantity
	}
	return nil
}

func chargeHostedBalanceTx(tx *gorm.DB, userId int, amountUSD float64) (int, error) {
	quota, clamp, err := HostedUSDToQuota(amountUSD)
	if err != nil {
		return 0, err
	}
	if clamp != nil {
		return 0, clamp
	}
	var user User
	if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
		return 0, err
	}
	if quota > 0 && user.Quota < quota {
		return 0, ErrHostedBalanceInsufficient
	}
	if quota > 0 {
		res := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, quota).
			Update("quota", gorm.Expr("quota - ?", quota))
		if res.Error != nil {
			return 0, res.Error
		}
		if res.RowsAffected != 1 {
			return 0, ErrHostedBalanceInsufficient
		}
	}
	return quota, nil
}

func insertHostedReceipt(tx *gorm.DB, userId int, planId int, amountUSD float64, quota int, kind string) (*SubscriptionOrder, error) {
	now := hostedNow()
	tradeNo := fmt.Sprintf("HOSTED%sUSR%dNO%s%d", strings.ToUpper(kind), userId, common.GetRandomString(6), time.Now().UnixNano())
	order := &SubscriptionOrder{
		UserId:          userId,
		PlanId:          planId,
		Money:           amountUSD,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodBalance,
		PaymentProvider: PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      now,
		CompleteTime:    now,
		ProviderPayload: fmt.Sprintf("hosted_kind=%s charged_quota=%d", kind, quota),
	}
	if err := tx.Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// PurchaseHostedChannelsWithBalance charges qty×SKU (or addon proration) and
// writes hosted_orders + hosted_entitlements. It never adds Quantity to UserSubscription.
func PurchaseHostedChannelsWithBalance(userId int, skuId int64, qty int, prorateDays int, kind, idempotencyKey, brandJSON string) (*HostedPurchaseResult, error) {
	if userId <= 0 || skuId <= 0 {
		return nil, errors.New("invalid userId or skuId")
	}
	if err := validateHostedQty(qty); err != nil {
		return nil, err
	}
	if kind == "" {
		kind = HostedOrderKindInitial
	}
	if kind != HostedOrderKindInitial && kind != HostedOrderKindAddon {
		return nil, errors.New("invalid hosted order kind")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = fmt.Sprintf("auto:%d:%d:%s", userId, skuId, common.GetRandomString(12))
	}

	if idempotencyKey != "" {
		if existing, err := GetHostedOrderByIdempotency(userId, idempotencyKey); err == nil {
			ent, _ := GetHostedEntitlement(userId, skuId)
			return &HostedPurchaseResult{Order: existing, Entitlement: ent, SubOrderID: existing.SubOrderID, Idempotent: true}, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	var result HostedPurchaseResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sku HostedSKU
		if err := lockForUpdate(tx).Where("id = ?", skuId).First(&sku).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrHostedSKUNotFound
			}
			return err
		}
		if !sku.Enabled {
			return ErrHostedSKUDisabled
		}
		if sku.PriceUSD < 0 {
			return ErrHostedInvalidAmount
		}

		var open int
		if err := tx.Model(&HostedOrder{}).
			Select("COALESCE(SUM(quantity - delivered_count), 0)").
			Where("user_id = ? AND sku_id = ? AND status IN ?", userId, sku.ID, []string{
				HostedOrderPaid, HostedOrderAllocated, HostedOrderPendingFulfillment,
			}).
			Scan(&open).Error; err != nil {
			return err
		}
		if open+qty > HostedMaxOpenQuantity {
			return ErrHostedOrderLimit
		}

		amount := sku.PriceUSD * float64(qty)
		if kind == HostedOrderKindAddon {
			if prorateDays <= 0 {
				return errors.New("addon requires remaining days")
			}
			var calcErr error
			amount, calcErr = HostedAddonAmountUSD(sku.PriceUSD, qty, prorateDays)
			if calcErr != nil {
				return calcErr
			}
		}
		quota, err := chargeHostedBalanceTx(tx, userId, amount)
		if err != nil {
			return err
		}

		receipt, err := insertHostedReceipt(tx, userId, sku.PlanId, amount, quota, kind)
		if err != nil {
			return err
		}

		now := hostedNow()
		order := &HostedOrder{
			UserID:         userId,
			SkuID:          sku.ID,
			SubOrderID:     receipt.Id,
			Kind:           kind,
			Quantity:       qty,
			Status:         HostedOrderPaid,
			ProrationDays:  prorateDays,
			DeadlineAt:     now + HostedSLASeconds(),
			IdempotencyKey: idempotencyKey,
			BrandConfig:    brandJSON,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}

		ent, err := GetOrCreateHostedEntitlementTx(tx, userId, sku.ID)
		if err != nil {
			return err
		}
		ent.Quantity += qty
		ent.Status = HostedEntitlementActive
		ent.UpdatedAt = now
		if kind == HostedOrderKindAddon && ent.ExpireAt > 0 {
			ent.GraceUntil = ent.ExpireAt + HostedGraceSeconds()
		}
		if err := tx.Save(ent).Error; err != nil {
			return err
		}

		result = HostedPurchaseResult{
			Order:       order,
			Entitlement: ent,
			SubOrderID:  receipt.Id,
			AmountUSD:   amount,
			Quota:       quota,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Quota > 0 {
		if err := cacheDecrUserQuota(userId, int64(result.Quota)); err != nil {
			common.SysLog("failed to decrease user quota cache after hosted purchase: " + err.Error())
		}
	}
	RecordLog(userId, LogTypeTopup, common.NewMessage("hosted {{kind}} qty={{quantity}} amount={{amount}} quota={{quota}} order={{order}}", map[string]any{"kind": kind, "quantity": qty, "amount": fmt.Sprintf("%.2f", result.AmountUSD), "quota": result.Quota, "order": result.Order.ID}))
	return &result, nil
}

// RenewHostedEntitlement charges quantity×unit and extends expire_at by one period.
func RenewHostedEntitlement(userId int, skuId int64, idempotencyKey string) (*HostedPurchaseResult, error) {
	if userId <= 0 || skuId <= 0 {
		return nil, errors.New("invalid userId or skuId")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = fmt.Sprintf("auto-renew:%d:%d:%s", userId, skuId, common.GetRandomString(12))
	}
	if idempotencyKey != "" {
		if existing, err := GetHostedOrderByIdempotency(userId, idempotencyKey); err == nil {
			ent, _ := GetHostedEntitlement(userId, skuId)
			return &HostedPurchaseResult{Order: existing, Entitlement: ent, SubOrderID: existing.SubOrderID, Idempotent: true}, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	var result HostedPurchaseResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sku HostedSKU
		if err := lockForUpdate(tx).Where("id = ?", skuId).First(&sku).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrHostedSKUNotFound
			}
			return err
		}
		if !sku.Enabled {
			return ErrHostedSKUDisabled
		}
		ent, err := GetOrCreateHostedEntitlementTx(tx, userId, sku.ID)
		if err != nil {
			return err
		}
		if ent.Quantity < 1 {
			return errors.New("no hosted quantity to renew")
		}
		amount := sku.PriceUSD * float64(ent.Quantity)
		quota, err := chargeHostedBalanceTx(tx, userId, amount)
		if err != nil {
			return err
		}
		receipt, err := insertHostedReceipt(tx, userId, sku.PlanId, amount, quota, HostedOrderKindRenew)
		if err != nil {
			return err
		}
		now := hostedNow()
		base := ent.ExpireAt
		if base < now {
			base = now
		}
		ent.ExpireAt = base + HostedPeriodSeconds()
		ent.GraceUntil = ent.ExpireAt + HostedGraceSeconds()
		ent.Status = HostedEntitlementActive
		ent.UpdatedAt = now
		if err := tx.Save(ent).Error; err != nil {
			return err
		}
		order := &HostedOrder{
			UserID:         userId,
			SkuID:          sku.ID,
			SubOrderID:     receipt.Id,
			Kind:           HostedOrderKindRenew,
			Quantity:       ent.Quantity,
			Status:         HostedOrderDelivered,
			DeliveredCount: ent.Quantity,
			AllocatedCount: ent.Quantity,
			IdempotencyKey: idempotencyKey,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		result = HostedPurchaseResult{Order: order, Entitlement: ent, SubOrderID: receipt.Id, AmountUSD: amount, Quota: quota}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Quota > 0 {
		if err := cacheDecrUserQuota(userId, int64(result.Quota)); err != nil {
			common.SysLog("failed to decrease user quota cache after hosted renew: " + err.Error())
		}
	}
	RecordLog(userId, LogTypeTopup, common.NewMessage("hosted renew qty={{quantity}} amount={{amount}} quota={{quota}}", map[string]any{"quantity": result.Order.Quantity, "amount": fmt.Sprintf("%.2f", result.AmountUSD), "quota": result.Quota}))
	return &result, nil
}

func MarkHostedExpireStarted(ent *HostedEntitlement, deliveredAt int64) error {
	if ent == nil {
		return ErrHostedEntitlementNotFound
	}
	if ent.ExpireAt > 0 {
		return nil
	}
	if deliveredAt <= 0 {
		deliveredAt = hostedNow()
	}
	ent.ExpireAt = deliveredAt + HostedPeriodSeconds()
	ent.GraceUntil = ent.ExpireAt + HostedGraceSeconds()
	ent.UpdatedAt = hostedNow()
	return DB.Model(ent).Updates(map[string]any{
		"expire_at":   ent.ExpireAt,
		"grace_until": ent.GraceUntil,
		"updated_at":  ent.UpdatedAt,
		"status":      HostedEntitlementActive,
	}).Error
}

func ApplyHostedGraceSweep(now int64) (int, error) {
	if now <= 0 {
		now = hostedNow()
	}
	res := DB.Model(&HostedEntitlement{}).
		Where("status = ? AND expire_at > 0 AND expire_at <= ? AND grace_until > ?", HostedEntitlementActive, now, now).
		Updates(map[string]any{"status": HostedEntitlementGrace, "updated_at": now})
	return int(res.RowsAffected), res.Error
}
