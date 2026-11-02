package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
)

// CheckGenerationDebt checks durable account debt before a new generation,
// independently of the selected funding source or a free-model shortcut.
func CheckGenerationDebt(info *relaycommon.RelayInfo) *types.NewAPIError {
	if info == nil {
		return types.NewError(fmt.Errorf("relay info is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if info.IsChannelTest {
		return nil
	}
	if err := model.CheckUserWalletDebt(info.UserId); err != nil {
		if errors.Is(err, model.ErrUserWalletDebt) {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	return nil
}

// CheckRealtimeGenerationQuota admits another event on an existing connection.
// The caller serializes this check with realtime partial settlement. The
// authenticated allowance bounds this connection even while native token
// decrements are pending in the batch queue or asynchronous cache updates.
func CheckRealtimeGenerationQuota(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	if apiErr := CheckGenerationDebt(info); apiErr != nil {
		return apiErr
	}
	if info.IsChannelTest || info.IsPlayground || info.TokenUnlimited {
		return nil
	}
	var originalQuota int
	var validQuota bool
	if c != nil {
		value, exists := c.Get("token_quota")
		originalQuota, validQuota = value.(int)
		validQuota = validQuota && exists
	}
	if !validQuota || originalQuota <= 0 || info.RealtimeConsumedQuota < 0 || info.RealtimeConsumedQuota >= originalQuota {
		return types.NewErrorWithStatusCode(fmt.Errorf("token quota is not enough"),
			types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	// Check both native snapshots: SQL must not be bypassed by a stale positive
	// cache, and pending cache reservations must not be bypassed by stale SQL.
	token, err := model.GetTokenById(info.TokenId)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	key := strings.TrimPrefix(info.TokenKey, "sk-")
	if token.UserId != info.UserId || token.Key != key {
		return types.NewError(fmt.Errorf("realtime token identity mismatch"), types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	remaining := token.RemainQuota
	if common.RedisEnabled {
		cachedToken, err := model.GetTokenByKey(key, false)
		if err != nil {
			return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		remaining = min(remaining, cachedToken.RemainQuota)
	}
	// Initial precharge already reduced the token balance. Credit only the part
	// this connection has not used; completed partial charges are not credit.
	held := max(info.FinalPreConsumedQuota-info.RealtimeConsumedQuota, 0)
	if remaining <= -held {
		return types.NewErrorWithStatusCode(fmt.Errorf("token quota is not enough"),
			types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	return nil
}

// PreConsumeBilling 根据用户计费偏好创建 BillingSession 并执行预扣费。
// 会话存储在 relayInfo.Billing 上，供后续 Settle / Refund 使用。
func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if relayInfo != nil && relayInfo.QuotaClamp != nil {
		return types.NewErrorWithStatusCode(
			relayInfo.QuotaClamp,
			types.ErrorCodeModelPriceError,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if preConsumedQuota < 0 {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("pre-consume quota cannot be negative: %d", preConsumedQuota),
			types.ErrorCodeModelPriceError,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

// ---------------------------------------------------------------------------
// SettleBilling — 后结算辅助函数
// ---------------------------------------------------------------------------

// SettleBilling 执行计费结算。如果 RelayInfo 上有 BillingSession 则通过 session 结算，
// 否则回退到旧的 PostConsumeQuota 路径（兼容按次计费等场景）。
func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, common.LogText("charging the difference after pre-consume: %s (actual %s, pre-consumed %s)",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, common.LogText("refunding the difference after pre-consume: %s (actual %s, pre-consumed %s)",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, common.LogText("pre-consumed quota matches the actual quota, no adjustment: %s (per-request billing)",
				logger.FormatQuota(actualQuota),
			))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}

		// 发送额度通知（订阅计费使用订阅剩余额度）
		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return nil
	}

	// 回退：无 BillingSession 时使用旧路径
	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta != 0 {
		return PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}
