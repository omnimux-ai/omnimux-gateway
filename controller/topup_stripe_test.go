package controller

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/waffo-com/waffo-go/core"
	"github.com/waffo-com/waffo-go/utils"
)

// A preset volume discount must reach the Stripe checkout: the wallet quotes the
// discounted money from getStripePayMoney, so the charged quantity has to be
// derived from the same number, otherwise the session charges list price while
// the order records the discounted one.
func TestStripeChargeUnitsFollowsPresetDiscount(t *testing.T) {
	originalUnitPrice := setting.StripeUnitPrice
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalDiscounts := make(map[int]float64, len(operation_setting.GetPaymentSetting().AmountDiscount))
	maps.Copy(originalDiscounts, operation_setting.GetPaymentSetting().AmountDiscount)
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()

	t.Cleanup(func() {
		setting.StripeUnitPrice = originalUnitPrice
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		operation_setting.GetPaymentSetting().AmountDiscount = originalDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	setting.StripeUnitPrice = 1
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{
		300:  0.95,
		1000: 0.9,
		3000: 0.85,
	}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))

	testCases := []struct {
		name      string
		amount    int64
		wantMoney float64
		wantUnits int64
	}{
		{name: "no preset discount charges list price", amount: 50, wantMoney: 50, wantUnits: 50},
		{name: "5 percent preset discount", amount: 300, wantMoney: 285, wantUnits: 285},
		{name: "10 percent preset discount", amount: 1000, wantMoney: 900, wantUnits: 900},
		{name: "15 percent preset discount", amount: 3000, wantMoney: 2550, wantUnits: 2550},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payMoney := getStripePayMoney(float64(tc.amount), "default")
			require.InDelta(t, tc.wantMoney, payMoney, 1e-9)
			require.Equal(t, tc.wantUnits, stripeChargeUnits(payMoney))
		})
	}
}

func TestStripeChargeUnitsScalesWithUnitPrice(t *testing.T) {
	originalUnitPrice := setting.StripeUnitPrice
	t.Cleanup(func() { setting.StripeUnitPrice = originalUnitPrice })

	testCases := []struct {
		name      string
		unitPrice float64
		payMoney  float64
		wantUnits int64
	}{
		{name: "dollar unit price", unitPrice: 1, payMoney: 285, wantUnits: 285},
		{name: "multi dollar unit price", unitPrice: 2.5, payMoney: 250, wantUnits: 100},
		{name: "invalid unit price falls back to one", unitPrice: 0, payMoney: 42, wantUnits: 42},
		{name: "never returns zero units", unitPrice: 100, payMoney: 10, wantUnits: 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setting.StripeUnitPrice = tc.unitPrice
			require.Equal(t, tc.wantUnits, stripeChargeUnits(tc.payMoney))
		})
	}
}

func TestPaymentTopUpCreditedQuotaAndManualComplete(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	oldQuotaPerUnit := common.QuotaPerUnit
	oldStripeUnitPrice := setting.StripeUnitPrice
	common.QuotaPerUnit = 500_000
	setting.StripeUnitPrice = 8.0
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		setting.StripeUnitPrice = oldStripeUnitPrice
	})

	user, _ := paymentTestBuyer(t, db)
	// C-1: Stripe order with UnitPrice=8.0 (Money=80.0 for Amount=10) must credit
	// 10 * 500,000 = 5,000,000 quota, never Money * 500,000 = 40,000,000.
	stripeOrder := &model.TopUp{
		UserId:              user.Id,
		Amount:              10,
		Money:               80.0,
		CreditedQuota:       5_000_000,
		ExpectedAmountMinor: 8000,
		ExpectedCurrency:    "CNY",
		ExpectedMode:        "live",
		ProviderSessionId:   "cs_live_expected",
		TradeNo:             "stripe-unitprice-c1",
		PaymentMethod:       model.PaymentMethodStripe,
		PaymentProvider:     model.PaymentProviderStripe,
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, stripeOrder.Insert())
	require.NoError(t, model.Recharge(stripeOrder.TradeNo, "cus_c1", "127.0.0.1", model.TopUpAnalyticsPayment{
		Currency: "CNY",
		Amount:   80.0,
		Verification: &model.PaymentVerification{
			AmountMinor: 8000,
			HasAmount:   true,
			Currency:    "CNY",
			Mode:        "live",
			SessionId:   "cs_live_expected",
		},
	}))

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 5_000_000, updated.Quota)

	// Legacy Stripe order (CreditedQuota=0) with UnitPrice=8.0 completed manually
	// must also credit Amount * QuotaPerUnit, not Money * QuotaPerUnit.
	legacyStripe := &model.TopUp{
		UserId:          user.Id,
		Amount:          2,
		Money:           16.0,
		TradeNo:         "stripe-legacy-manual",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, legacyStripe.Insert())
	require.NoError(t, model.ManualCompleteTopUp(legacyStripe.TradeNo, "127.0.0.1"))

	// M-2: Creem ManualCompleteTopUp must credit Amount (already quota), never Amount * 500,000.
	creemManual := &model.TopUp{
		UserId:          user.Id,
		Amount:          500_000,
		Money:           1.0,
		TradeNo:         "creem-manual-quota",
		PaymentMethod:   model.PaymentMethodCreem,
		PaymentProvider: model.PaymentProviderCreem,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, creemManual.Insert())
	require.NoError(t, model.ManualCompleteTopUp(creemManual.TradeNo, "127.0.0.1"))

	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 5_000_000+1_000_000+500_000, updated.Quota)
}

func TestPaymentWebhookVerificationRejectsTamperedCallbacks(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	configureStripePaymentTest(t)
	configureWaffoPaymentTest(t)
	user, plan := paymentTestBuyer(t, db)

	oldCreemKey, oldCreemProducts, oldCreemSecret, oldCreemTest := setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret, setting.CreemTestMode
	oldPayAddr, oldEpayID, oldEpayKey, oldPayMethods := operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey, operation_setting.PayMethods
	t.Cleanup(func() {
		setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret, setting.CreemTestMode = oldCreemKey, oldCreemProducts, oldCreemSecret, oldCreemTest
		operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey, operation_setting.PayMethods = oldPayAddr, oldEpayID, oldEpayKey, oldPayMethods
	})

	// 1. Stripe: reject underpayment, currency mismatch, test-mode replay, wrong session, and mode confusion.
	stripeTopUp := &model.TopUp{
		UserId:              user.Id,
		Amount:              10,
		Money:               10,
		CreditedQuota:       5_000_000,
		ExpectedAmountMinor: 1000,
		ExpectedCurrency:    "USD",
		ExpectedMode:        "live",
		ProviderSessionId:   "cs_live_1",
		TradeNo:             "stripe-verify-topup",
		PaymentMethod:       model.PaymentMethodStripe,
		PaymentProvider:     model.PaymentProviderStripe,
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, stripeTopUp.Insert())

	postStripeCustom := func(livemode bool, obj gin.H) *httptest.ResponseRecorder {
		body, err := common.Marshal(gin.H{
			"id":       "evt_verify",
			"object":   "event",
			"livemode": livemode,
			"type":     stripe.EventTypeCheckoutSessionCompleted,
			"data":     gin.H{"object": obj},
		})
		require.NoError(t, err)
		sig := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: body, Secret: setting.StripeWebhookSecret}).Header
		return postPaymentTestEvent(StripeWebhook, string(body), "Stripe-Signature", sig)
	}

	tamperedStripe := []gin.H{
		{"id": "cs_live_1", "mode": "payment", "client_reference_id": stripeTopUp.TradeNo, "customer": "cus_1", "amount_total": 100, "currency": "usd", "status": "complete", "payment_status": "paid"},
		{"id": "cs_live_1", "mode": "payment", "client_reference_id": stripeTopUp.TradeNo, "customer": "cus_1", "amount_total": 1000, "currency": "jpy", "status": "complete", "payment_status": "paid"},
		{"id": "cs_live_other", "mode": "payment", "client_reference_id": stripeTopUp.TradeNo, "customer": "cus_1", "amount_total": 1000, "currency": "usd", "status": "complete", "payment_status": "paid"},
		{"id": "cs_live_1", "mode": "subscription", "client_reference_id": stripeTopUp.TradeNo, "customer": "cus_1", "amount_total": 1000, "currency": "usd", "status": "complete", "payment_status": "paid"},
	}
	for _, obj := range tamperedStripe {
		postStripeCustom(true, obj)
		status, _ := paymentTestState(t, false, stripeTopUp.TradeNo)
		assert.Equal(t, common.TopUpStatusPending, status)
	}
	// Test-mode event against live order must also fail.
	postStripeCustom(false, gin.H{"id": "cs_live_1", "mode": "payment", "client_reference_id": stripeTopUp.TradeNo, "customer": "cus_1", "amount_total": 1000, "currency": "usd", "status": "complete", "payment_status": "paid"})
	status, _ := paymentTestState(t, false, stripeTopUp.TradeNo)
	assert.Equal(t, common.TopUpStatusPending, status)

	// 2. Creem: reject test-mode replay, wrong product, and underpayment; fail closed on empty secret.
	setting.CreemApiKey = "creem_key"
	setting.CreemProducts = `[{"productId":"prod_big","name":"Big","price":100,"currency":"USD","quota":50000000}]`
	setting.CreemWebhookSecret = "creem_wh_secret"
	setting.CreemTestMode = false
	assert.False(t, verifyCreemSignature("payload", "sig", ""), "empty secret must fail closed even in test mode")
	setting.CreemTestMode = true
	assert.False(t, verifyCreemSignature("payload", "sig", ""), "empty secret must fail closed in test mode")
	setting.CreemTestMode = false

	creemOrder := &model.TopUp{
		UserId:              user.Id,
		Amount:              50_000_000,
		Money:               100,
		CreditedQuota:       50_000_000,
		ExpectedAmountMinor: 10000,
		ExpectedCurrency:    "USD",
		ExpectedMode:        "live",
		ProviderProductId:   "prod_big",
		TradeNo:             "creem-verify-topup",
		PaymentMethod:       model.PaymentMethodCreem,
		PaymentProvider:     model.PaymentProviderCreem,
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, creemOrder.Insert())
	for _, payload := range []string{
		fmt.Sprintf(`{"id":"e1","eventType":"checkout.completed","object":{"request_id":%q,"mode":"test","product":{"id":"prod_big"},"order":{"id":"o1","status":"paid","type":"onetime","currency":"USD","amount_paid":10000}}}`, creemOrder.TradeNo),
		fmt.Sprintf(`{"id":"e2","eventType":"checkout.completed","object":{"request_id":%q,"mode":"live","product":{"id":"prod_cheap"},"order":{"id":"o2","status":"paid","type":"onetime","currency":"USD","amount_paid":10000}}}`, creemOrder.TradeNo),
		fmt.Sprintf(`{"id":"e3","eventType":"checkout.completed","object":{"request_id":%q,"mode":"live","product":{"id":"prod_big"},"order":{"id":"o3","status":"paid","type":"onetime","currency":"USD","amount_paid":100}}}`, creemOrder.TradeNo),
	} {
		rec := postPaymentTestEvent(CreemWebhook, payload, CreemSignatureHeader, generateCreemSignature(payload, setting.CreemWebhookSecret))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		st, _ := paymentTestState(t, false, creemOrder.TradeNo)
		assert.Equal(t, common.TopUpStatusPending, st)
	}

	// 3. Waffo: reject underpayment and currency mismatch.
	waffoOrder := &model.TopUp{
		UserId:              user.Id,
		Amount:              10,
		Money:               10,
		CreditedQuota:       5_000_000,
		ExpectedAmountMinor: 1000,
		ExpectedCurrency:    "USD",
		TradeNo:             "waffo-verify-topup",
		PaymentMethod:       model.PaymentMethodWaffo,
		PaymentProvider:     model.PaymentProviderWaffo,
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, waffoOrder.Insert())
	for _, note := range []core.PaymentNotificationResult{
		{MerchantOrderID: waffoOrder.TradeNo, OrderStatus: core.OrderStatusPaySuccess, OrderCurrency: "USD", OrderAmount: "1.00"},
		{MerchantOrderID: waffoOrder.TradeNo, OrderStatus: core.OrderStatusPaySuccess, OrderCurrency: "JPY", OrderAmount: "1000"},
	} {
		body, err := common.Marshal(core.PaymentNotification{EventType: core.EventPayment, Result: &note})
		require.NoError(t, err)
		sig, err := utils.Sign(string(body), setting.WaffoPrivateKey)
		require.NoError(t, err)
		postPaymentTestEvent(WaffoWebhook, string(body), core.HeaderSignature, sig)
		st, _ := paymentTestState(t, false, waffoOrder.TradeNo)
		assert.Equal(t, common.TopUpStatusPending, st)
	}

	// 4. Epay subscription: reject when webhook disabled and reject underpayment.
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "epay_secret"
	operation_setting.PayMethods = nil // disabled
	subOrder := &model.SubscriptionOrder{
		UserId:              user.Id,
		PlanId:              plan.Id,
		Money:               10,
		ExpectedAmountMinor: 1000,
		TradeNo:             "sub-epay-verify",
		PaymentMethod:       "alipay",
		PaymentProvider:     model.PaymentProviderEpay,
		Status:              common.TopUpStatusPending,
	}
	require.NoError(t, subOrder.Insert())

	buildEpayReq := func(money, pid string) *http.Request {
		sign := epay.MD5String(fmt.Sprintf("money=%s&name=sub&out_trade_no=%s&pid=%s&trade_status=%s&type=alipay", money, subOrder.TradeNo, pid, epay.StatusTradeSuccess), operation_setting.EpayKey)
		q := url.Values{"pid": {pid}, "type": {"alipay"}, "money": {money}, "name": {"sub"}, "out_trade_no": {subOrder.TradeNo}, "trade_status": {epay.StatusTradeSuccess}, "sign_type": {"MD5"}, "sign": {sign}}
		return httptest.NewRequest(http.MethodGet, "/api/subscription/epay/notify?"+q.Encode(), nil)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = buildEpayReq("10.00", "epay_id")
	SubscriptionEpayNotify(ctx)
	assert.Equal(t, "fail", rec.Body.String())

	// Enable Epay, then send underpaid callback ("0.01" vs expected "10.00").
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	ctx.Request = buildEpayReq("0.01", "epay_id")
	SubscriptionEpayNotify(ctx)
	assert.Equal(t, "fail", rec.Body.String())
	st, _ := paymentTestState(t, true, subOrder.TradeNo)
	assert.Equal(t, common.TopUpStatusPending, st)
}
