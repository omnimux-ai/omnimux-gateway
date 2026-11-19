package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/analytics"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopUpPaidEmitsOnlyForCreditedOrder(t *testing.T) {
	truncateTables(t)
	original := trackTopUpPaidEvent
	t.Cleanup(func() { trackTopUpPaidEvent = original })
	var events []analytics.Event
	trackTopUpPaidEvent = func(event analytics.Event) { events = append(events, event) }
	user := insertUserForPaymentGuardTest(t, 9201, 0)
	tests := []struct {
		provider, currency string
		recharge           func(tradeNo string, payment TopUpAnalyticsPayment) error
	}{
		{PaymentProviderEpay, "", func(tradeNo string, payment TopUpAnalyticsPayment) error {
			_, err := RechargeEpay(tradeNo, "alipay", "127.0.0.1", payment)
			return err
		}},
		{PaymentProviderStripe, "USD", func(tradeNo string, payment TopUpAnalyticsPayment) error {
			return Recharge(tradeNo, "", "127.0.0.1", payment)
		}},
		{PaymentProviderCreem, "EUR", func(tradeNo string, payment TopUpAnalyticsPayment) error {
			return RechargeCreem(tradeNo, "", "", "127.0.0.1", payment)
		}},
		{PaymentProviderWaffo, "JPY", func(tradeNo string, payment TopUpAnalyticsPayment) error {
			return RechargeWaffo(tradeNo, "127.0.0.1", payment)
		}},
		{PaymentProviderWaffoPancake, "USD", func(tradeNo string, payment TopUpAnalyticsPayment) error {
			return RechargeWaffoPancake(tradeNo, payment)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			tradeNo := "topup-event-" + tc.provider
			insertTopUpForPaymentGuardTest(t, tradeNo, user.Id, tc.provider)
			payment := TopUpAnalyticsPayment{Amount: 8.5, Currency: tc.currency}
			events = nil
			quotaBefore := getUserQuotaForPaymentGuardTest(t, user.Id)
			require.NoError(t, tc.recharge(tradeNo, payment))
			require.Len(t, events, 1)
			quotaAfter := getUserQuotaForPaymentGuardTest(t, user.Id)
			assert.Greater(t, quotaAfter, quotaBefore)
			order := GetTopUpByTradeNo(tradeNo)
			require.NotNil(t, order)
			assert.Equal(t, common.TopUpStatusSuccess, order.Status)
			assert.Equal(t, analytics.Event{
				Name:     analytics.EventTopUpPaid,
				UserID:   user.Id,
				OrderRef: tradeNo,
				Data: map[string]any{
					"amount":         8.5,
					"currency":       tc.currency,
					"payment_method": order.PaymentMethod,
					"paid_at":        order.CompleteTime,
				},
			}, events[0])

			_ = tc.recharge(tradeNo, payment)
			assert.Len(t, events, 1)
			assert.Equal(t, quotaAfter, getUserQuotaForPaymentGuardTest(t, user.Id))
		})
	}

	events = nil
	insertTopUpForPaymentGuardTest(t, "uncredited-order", user.Id+1, PaymentProviderEpay)
	_, err := RechargeEpay("uncredited-order", "alipay", "127.0.0.1")
	require.Error(t, err)
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "uncredited-order"))
	assert.Empty(t, events)

	insertTopUpForPaymentGuardTest(t, "manual-order", user.Id, PaymentProviderEpay)
	require.NoError(t, ManualCompleteTopUp("manual-order", "127.0.0.1"))
	assert.Empty(t, events)
}
