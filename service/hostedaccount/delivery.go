package hostedaccount

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// DeliverPending assigns available pool phones to a paid/pending order.
// Expire clock starts on the first delivered channel of the entitlement.
func (s *Service) DeliverPending(_ int, orderID int64) error {
	var order model.HostedOrder
	if err := model.DB.First(&order, orderID).Error; err != nil {
		return err
	}
	need := order.Quantity - order.DeliveredCount
	if need <= 0 {
		return nil
	}
	sku, err := model.GetHostedSKU(order.SkuID)
	if err != nil {
		return err
	}
	delivered := 0
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var phones []*model.HostedAccount
		q := lockForUpdateAccounts(tx).
			Where("platform = ? AND region = ? AND status = ? AND allocatable = ? AND assigned_user_id = 0",
				sku.Platform, sku.Region, model.HostedAccountAvailable, true).
			Order("id asc").
			Limit(need)
		if err := q.Find(&phones).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		for _, phone := range phones {
			res := tx.Model(phone).
				Where("id = ? AND status = ? AND assigned_user_id = 0", phone.ID, model.HostedAccountAvailable).
				Updates(map[string]any{
					"status":           model.HostedAccountAssigned,
					"assigned_user_id": order.UserID,
					"assigned_sub_id":  order.SubOrderID,
					"brand_config":     order.BrandConfig,
					"warmup_status":    model.HostedWarmupNone,
					"readiness":        0,
					"updated_at":       now,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				continue
			}
			alloc := &model.HostedAllocation{
				OrderID:     order.ID,
				AccountID:   phone.ID,
				State:       model.HostedAllocDelivered,
				ReservedAt:  now,
				DeliveredAt: now,
			}
			if err := tx.Create(alloc).Error; err != nil {
				return err
			}
			plan := &model.HostedWarmupPlan{
				AccountID:     phone.ID,
				PlaybookCode:  "warmup7_v1",
				BrandKeywords: order.BrandConfig,
				CurrentDay:    1,
				State:         "generating",
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := tx.Create(plan).Error; err != nil {
				return err
			}
			if err := tx.Model(phone).Updates(map[string]any{
				"status":        model.HostedAccountWarmingUp,
				"warmup_status": model.HostedWarmupInProgress,
				"updated_at":    now,
			}).Error; err != nil {
				return err
			}
			delivered++
		}
		order.DeliveredCount += delivered
		order.AllocatedCount += delivered
		order.UpdatedAt = now
		switch {
		case order.DeliveredCount >= order.Quantity:
			order.Status = model.HostedOrderDelivered
		case order.DeliveredCount > 0:
			order.Status = model.HostedOrderAllocated
		default:
			order.Status = model.HostedOrderPendingFulfillment
		}
		return tx.Save(&order).Error
	})
	if err != nil {
		return err
	}
	if delivered > 0 {
		ent, err := model.GetHostedEntitlement(order.UserID, order.SkuID)
		if err == nil {
			_ = model.MarkHostedExpireStarted(ent, common.GetTimestamp())
		}
	}
	return nil
}

func (s *Service) DeliverOpenOrders() (int, error) {
	var orders []*model.HostedOrder
	err := model.DB.Where("status IN ?", []string{
		model.HostedOrderPaid, model.HostedOrderAllocated, model.HostedOrderPendingFulfillment,
	}).Find(&orders).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, o := range orders {
		if err := s.DeliverPending(o.UserID, o.ID); err == nil {
			n++
		}
	}
	return n, nil
}

func lockForUpdateAccounts(tx *gorm.DB) *gorm.DB {
	return model.LockForUpdate(tx)
}
