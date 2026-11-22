package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/shopspring/decimal"
	waffo "github.com/waffo-com/waffo-go"
	"github.com/waffo-com/waffo-go/core"
	waffonet "github.com/waffo-com/waffo-go/net"
	"github.com/waffo-com/waffo-go/types/order"
)

const (
	paymentOrderExpiryBatchSize = 50
	paymentOrderInquiryTimeout  = 10 * time.Second
)

type paymentOrderExpiryHandler struct{}

func (paymentOrderExpiryHandler) Type() string { return model.SystemTaskTypePaymentOrderExpiry }

func (paymentOrderExpiryHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("PAYMENT_ORDER_EXPIRY_TASK_ENABLED", false)
}

func (paymentOrderExpiryHandler) Interval() time.Duration {
	minutes := common.GetEnvOrDefault("PAYMENT_ORDER_EXPIRY_TASK_INTERVAL_MINUTES", 60)
	if minutes < 1 || int64(minutes) > math.MaxInt64/int64(time.Minute) {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

func (paymentOrderExpiryHandler) NewPayload() any { return nil }

func paymentOrderExpiryCutoff(now int64) int64 {
	const secondsPerHour = int64(time.Hour / time.Second)
	hours := max(common.GetEnvOrDefault("PAYMENT_ORDER_EXPIRY_MIN_AGE_HOURS", 72), int(model.PaymentOrderExpiryMinAge/time.Hour))
	if int64(hours) > now/secondsPerHour {
		return 0
	}
	return now - int64(hours)*secondsPerHour
}

type paymentOrderExpiryCounts struct {
	Checked     int `json:"checked"`
	Expired     int `json:"expired"`
	Unconfirmed int `json:"unconfirmed"`
	QueryErrors int `json:"query_errors"`
}

type paymentOrderExpiryResult struct {
	TopUps        paymentOrderExpiryCounts `json:"top_ups"`
	Subscriptions paymentOrderExpiryCounts `json:"subscriptions"`
}

func (h paymentOrderExpiryHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	var result paymentOrderExpiryResult
	if !h.Enabled() {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, result, nil)
		return
	}

	var waffoClient *waffo.Waffo
	if isWaffoWebhookConfigured() {
		cfg, err := getWaffoConfig()
		if err != nil {
			logger.LogWarn(ctx, common.LogText("Payment order expiry could not initialize Waffo inquiry task_id=%s", task.TaskID))
		} else {
			cfg.CustomTransport = waffoOrderInquiryTransport{}
			waffoClient = waffo.New(cfg)
		}
	}

	cutoff := paymentOrderExpiryCutoff(common.GetTimestamp())
	for _, subscription := range []bool{false, true} {
		counts := &result.TopUps
		if subscription {
			counts = &result.Subscriptions
		}
		if err := reconcileAgedPaymentOrders(ctx, task, runnerID, subscription, cutoff, waffoClient, counts); err != nil {
			finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, result, err)
			return
		}
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, result, nil)
}

func reconcileAgedPaymentOrders(ctx context.Context, task *model.SystemTask, runnerID string, subscription bool, cutoff int64, waffoClient *waffo.Waffo, counts *paymentOrderExpiryCounts) error {
	var afterID int
	for {
		orders, err := model.ListPaymentOrderExpiryCandidates(ctx, subscription, afterID, cutoff, paymentOrderExpiryBatchSize)
		if err != nil {
			return err
		}
		for _, candidate := range orders {
			if err := ctx.Err(); err != nil {
				return err
			}
			afterID = candidate.ID
			counts.Checked++

			queryCtx, cancel := context.WithTimeout(ctx, paymentOrderInquiryTimeout)
			var closed bool
			var queryErr error
			switch candidate.PaymentProvider {
			case model.PaymentProviderWaffo:
				if waffoClient != nil {
					closed, queryErr = inquireClosedWaffoOrder(queryCtx, waffoClient, candidate)
				}
			case model.PaymentProviderWaffoPancake:
				closed, queryErr = service.QueryCanceledWaffoPancakeOrder(queryCtx, candidate.TradeNo, candidate.UserID, subscription)
			}
			cancel()
			if queryErr != nil {
				counts.QueryErrors++
				logger.LogWarn(ctx, common.LogText("Payment order expiry inquiry failed task_id=%s payment_provider=%s trade_no=%s error=%q",
					task.TaskID, candidate.PaymentProvider, candidate.TradeNo, queryErr.Error()))
			}
			if queryErr != nil || !closed {
				counts.Unconfirmed++
				continue
			}
			expired, err := model.ExpireConfirmedUnpaidOrder(ctx, task.TaskID, runnerID, subscription, candidate, cutoff)
			if err != nil {
				return err
			}
			if expired {
				counts.Expired++
			}
		}
		if len(orders) < paymentOrderExpiryBatchSize {
			return ctx.Err()
		}
	}
}

func inquireClosedWaffoOrder(ctx context.Context, client *waffo.Waffo, candidate model.PaymentOrderExpiryCandidate) (bool, error) {
	resp, err := client.Order().Inquiry(ctx, &order.InquiryOrderParams{PaymentRequestID: candidate.TradeNo}, nil)
	if err != nil || resp == nil || !resp.IsSuccess() {
		return false, errors.New("Waffo order inquiry did not succeed")
	}
	data := resp.GetData()
	if data == nil || data.OrderStatus != core.OrderStatusOrderClose || data.PaymentRequestID != candidate.TradeNo || data.MerchantOrderID != candidate.TradeNo || data.AcquiringOrderID == "" {
		return false, nil
	}
	if merchant := client.GetConfig().MerchantID; merchant != "" && data.MerchantInfo != nil && data.MerchantInfo.MerchantID != "" && data.MerchantInfo.MerchantID != merchant {
		return false, nil
	}
	if data.UserInfo != nil && data.UserInfo.UserID != "" && data.UserInfo.UserID != strconv.Itoa(candidate.UserID) {
		return false, nil
	}
	if data.FinalDealAmount != "" {
		amount, err := decimal.NewFromString(data.FinalDealAmount)
		if err != nil || !amount.IsZero() {
			return false, nil
		}
	}
	return true, nil
}

// The SDK signs and verifies the inquiry. Its default transport follows
// redirects, and its decoder ignores HTTP status, so this inquiry transport
// accepts only a direct successful response before giving it to the SDK.
type waffoOrderInquiryTransport struct{}

func (waffoOrderInquiryTransport) Send(ctx context.Context, req *waffonet.HttpRequest) (*waffonet.HttpResponse, error) {
	request, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for key, value := range req.Headers {
		request.Header.Set(key, value)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Waffo inquiry returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(http.MaxBytesReader(nil, resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return waffonet.NewHttpResponse(resp.StatusCode, map[string]string{core.HeaderSignature: resp.Header.Get(core.HeaderSignature)}, body), nil
}
