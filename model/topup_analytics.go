package model

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/pkg/analytics"
)

// TopUpAnalyticsPayment carries optional facts from a verified payment callback.
// Amount is the positive paid amount in major units of Currency.
type TopUpAnalyticsPayment struct {
	Currency string
	Amount   float64
}

var trackTopUpPaidEvent = analytics.Track

// trackTopUpPaid runs after the callback transaction that first credits the order commits.
func trackTopUpPaid(topUp *TopUp, payment ...TopUpAnalyticsPayment) {
	amount := topUp.Money
	currency := ""
	if len(payment) > 0 {
		currency = strings.ToUpper(strings.TrimSpace(payment[0].Currency))
		paidAmount := payment[0].Amount
		if paidAmount > 0 && !math.IsInf(paidAmount, 0) {
			amount = paidAmount
		}
	}
	trackTopUpPaidEvent(analytics.Event{
		Name:     analytics.EventTopUpPaid,
		UserID:   topUp.UserId,
		OrderRef: topUp.TradeNo,
		Data: map[string]any{
			"amount":         amount,
			"currency":       currency,
			"payment_method": topUp.PaymentMethod,
			"paid_at":        topUp.CompleteTime,
		},
	})
}
