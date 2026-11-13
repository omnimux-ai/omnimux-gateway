package service

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWaffoPancakePaymentBindingAndSettlement(t *testing.T) {
	oldDB, oldLogDB, oldStore := model.DB, model.LOG_DB, setting.WaffoPancakeStoreID
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "payments.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB, model.LOG_DB, setting.WaffoPancakeStoreID = db, db, "STO_local"
	common.RedisEnabled, common.BatchUpdateEnabled = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB, setting.WaffoPancakeStoreID = oldDB, oldLogDB, oldStore
		common.RedisEnabled, common.BatchUpdateEnabled = oldRedis, oldBatch
		model.InvalidateSubscriptionPlanCache(1)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.SubscriptionOrder{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "payment-fixture", Quota: 0}).Error)
	require.NoError(t, db.Create(&model.SubscriptionPlan{Id: 1, Title: "local plan", PriceAmount: 99, DurationUnit: "month", DurationValue: 1, QuotaResetPeriod: model.SubscriptionResetNever, TotalAmount: 100}).Error)
	model.InvalidateSubscriptionPlanCache(1)
	for _, subscription := range []bool{false, true} {
		name := "wallet"
		if subscription {
			name = "subscription"
		}
		t.Run(name, func(t *testing.T) {
			ref := "local-" + name
			if subscription {
				require.NoError(t, db.Create(&model.SubscriptionOrder{UserId: 1, PlanId: 1, Money: 10.005, TradeNo: ref, PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending}).Error)
			} else {
				require.NoError(t, db.Create(&model.TopUp{UserId: 1, Amount: 1, Money: 10.005, TradeNo: ref, PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending}).Error)
			}
			resolve := ResolveWaffoPancakeTradeNo
			if subscription {
				resolve = ResolveWaffoPancakeSubscriptionTradeNo
			}
			settle := func(trade string) error {
				if subscription {
					return model.CompleteSubscriptionOrder(trade, "", model.PaymentProviderWaffoPancake, "")
				}
				return model.RechargeWaffoPancake(trade)
			}
			base := WaffoPancakeWebhookEvent{EventType: "order.completed", Mode: "prod", StoreID: "STO_local", Data: WaffoPancakeWebhookData{OrderMerchantExternalID: ref, MerchantProvidedBuyerIdentity: WaffoPancakeBuyerIdentityFromUserID(1), Currency: "USD", Amount: "11.0100", TaxAmount: "1.00"}}
			cases := []struct {
				name   string
				change func(*WaffoPancakeWebhookEvent)
			}{
				{"other store", func(e *WaffoPancakeWebhookEvent) { e.StoreID = "STO_other" }},
				{"missing store", func(e *WaffoPancakeWebhookEvent) { e.StoreID = "" }},
				{"test mode", func(e *WaffoPancakeWebhookEvent) { e.Mode = "test" }},
				{"missing mode", func(e *WaffoPancakeWebhookEvent) { e.Mode = "" }},
				{"other currency", func(e *WaffoPancakeWebhookEvent) { e.Data.Currency = "EUR" }},
				{"wrong buyer", func(e *WaffoPancakeWebhookEvent) { e.Data.MerchantProvidedBuyerIdentity = "other" }},
				{"missing reference", func(e *WaffoPancakeWebhookEvent) { e.Data.OrderMerchantExternalID = "" }},
				{"underpaid", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "2.00" }},
				{"gross equals expected but tax underpays", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "10.01" }},
				{"overpaid", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "12.00" }},
				{"negative tax", func(e *WaffoPancakeWebhookEvent) { e.Data.TaxAmount = "-1" }},
				{"tax exceeds gross", func(e *WaffoPancakeWebhookEvent) { e.Data.TaxAmount = "12" }},
				{"missing tax", func(e *WaffoPancakeWebhookEvent) { e.Data.TaxAmount = "" }},
				{"missing amount", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "" }},
				{"zero amount", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "0" }},
				{"negative amount", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "-1" }},
				{"exponent", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "11.01e0" }},
				{"oversized", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = strings.Repeat("1", 33) }},
				{"nonfinite", func(e *WaffoPancakeWebhookEvent) { e.Data.Amount = "NaN" }},
			}
			var beforeUser model.User
			require.NoError(t, db.First(&beforeUser, 1).Error)
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					e := base
					tc.change(&e)
					trade, err := resolve(&e)
					require.Error(t, err)
					assert.Empty(t, trade)
				})
			}
			setting.WaffoPancakeStoreID = ""
			_, err := resolve(&base)
			require.Error(t, err)
			setting.WaffoPancakeStoreID = "STO_local"
			_, err = resolve(nil)
			require.Error(t, err)
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			assert.Equal(t, beforeUser.Quota, user.Quota)
			var count int64
			require.NoError(t, db.Model(&model.UserSubscription{}).Count(&count).Error)
			assert.Zero(t, count)
			if subscription {
				assert.Equal(t, common.TopUpStatusPending, model.GetSubscriptionOrderByTradeNo(ref).Status)
			} else {
				assert.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo(ref).Status)
			}
			for i := 0; i < 2; i++ {
				trade, err := resolve(&base)
				require.NoError(t, err)
				require.Equal(t, ref, trade)
				require.NoError(t, settle(trade))
			}
			require.NoError(t, db.First(&user, 1).Error)
			if subscription {
				assert.Equal(t, beforeUser.Quota, user.Quota)
				require.NoError(t, db.Model(&model.UserSubscription{}).Count(&count).Error)
				assert.EqualValues(t, 1, count)
				assert.Equal(t, common.TopUpStatusSuccess, model.GetSubscriptionOrderByTradeNo(ref).Status)
			} else {
				assert.Equal(t, int(common.QuotaPerUnit), user.Quota)
				assert.Equal(t, common.TopUpStatusSuccess, model.GetTopUpByTradeNo(ref).Status)
			}
		})
	}
}

func TestWaffoPancakeExpectedPriceValidation(t *testing.T) {
	oldStore := setting.WaffoPancakeStoreID
	setting.WaffoPancakeStoreID = "STO_local"
	t.Cleanup(func() { setting.WaffoPancakeStoreID = oldStore })
	event := &WaffoPancakeWebhookEvent{Mode: "prod", StoreID: "STO_local", Data: WaffoPancakeWebhookData{Currency: "USD", Amount: "10.00", TaxAmount: "0"}}
	require.NoError(t, validateWaffoPancakePayment(event, 10))
	for _, money := range []float64{0, -1, 0.001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		assert.Error(t, validateWaffoPancakePayment(event, money))
	}
	// No fabricated signatures or key overrides: unsigned input never authenticates.
	_, err := VerifyConfiguredWaffoPancakeWebhook(`{"mode":"prod"}`, "")
	require.Error(t, err)
}
