package controller

import (
	"maps"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
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
