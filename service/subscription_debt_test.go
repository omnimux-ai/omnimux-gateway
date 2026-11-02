package service

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionDebtBilling(t *testing.T, wallet int, total int64) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	require.NoError(t, i18n.Init())
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.BatchUpdateEnabled = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled = oldRedis, oldBatch
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}, &model.Token{}, &model.Task{}, &model.TopUp{}, &model.Log{}, &model.Channel{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "debt-fixture", Quota: wallet}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "debt-fixture-token", RemainQuota: 1000, Status: common.TokenStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.SubscriptionPlan{Id: 1, Title: "Debt fixture", QuotaResetPeriod: model.SubscriptionResetNever}).Error)
	require.NoError(t, db.Create(&model.UserSubscription{Id: 1, UserId: 1, PlanId: 1, AmountTotal: total, AmountUsed: 850, Status: "active", StartTime: time.Now().Unix() - 60, EndTime: time.Now().Unix() + 3600}).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/test", nil)
	info := &relaycommon.RelayInfo{UserId: 1, TokenId: 1, TokenKey: "debt-fixture-token", RequestId: t.Name(), UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
	return c, info
}

func TestSubscriptionDebtFinalSettlementAndNormalControls(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		wallet, actual, wantWallet      int
		total, wantUsed, wantWalletPart int64
	}{
		{"overrun creates debt", 0, 200, -50, 1000, 1000, 50},
		{"wallet covers overrun", 100, 200, 50, 1000, 1000, 50},
		{"within allowance", 0, 120, 0, 1000, 970, 0},
		{"unused reservation refunded", 0, 50, 0, 1000, 900, 0},
		{"unlimited subscription", 0, 200, 0, 0, 1050, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info := setupSubscriptionDebtBilling(t, tc.wallet, tc.total)
			session, apiErr := NewBillingSession(c, info, 100)
			require.Nil(t, apiErr)
			require.NoError(t, session.Settle(tc.actual))
			require.NoError(t, session.Settle(tc.actual))
			var user model.User
			var sub model.UserSubscription
			var token model.Token
			require.NoError(t, model.DB.First(&user, 1).Error)
			require.NoError(t, model.DB.First(&sub, 1).Error)
			require.NoError(t, model.DB.First(&token, 1).Error)
			assert.Equal(t, tc.wantWallet, user.Quota)
			assert.Equal(t, tc.wantUsed, sub.AmountUsed)
			assert.Equal(t, tc.actual, token.UsedQuota)
			assert.Equal(t, 1000-tc.actual, token.RemainQuota)
			assert.Equal(t, tc.wantWalletPart, info.SubscriptionWalletConsumed)
			assert.Equal(t, int64(tc.actual)-tc.wantWalletPart, info.SubscriptionPreConsumed+info.SubscriptionPostDelta)
			assert.False(t, session.NeedsRefund())
		})
	}
}

func TestSubscriptionDebtBlocksAllFundingPreferencesAndRenewal(t *testing.T) {
	c, info := setupSubscriptionDebtBilling(t, -50, 1000)
	for _, preference := range []string{"subscription_only", "subscription_first", "wallet_first", "wallet_only"} {
		info.UserSetting.BillingPreference = preference
		_, apiErr := NewBillingSession(c, info, 1)
		require.NotNil(t, apiErr, preference)
		assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	}
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = 1").Update("amount_used", 0).Error)
	assert.NotNil(t, CheckGenerationDebt(info), "reset allowance must not erase wallet debt")
	var token model.Token
	require.NoError(t, model.DB.First(&token, 1).Error)
	assert.Equal(t, 1000, token.RemainQuota, "rejected requests must not reserve tokens")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = 1").Update("quota", 0).Error)
	info.UserSetting.BillingPreference = "subscription_only"
	_, apiErr := NewBillingSession(c, info, 1)
	require.Nil(t, apiErr, "zero-wallet prepaid customers have no outstanding debt")
}

func TestSubscriptionDebtRechargeOffsetsDebtAndCreditsOrderOnce(t *testing.T) {
	c, info := setupSubscriptionDebtBilling(t, 0, 1000)
	session, apiErr := NewBillingSession(c, info, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(200))
	oldQuotaUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaUnit })
	for _, tc := range []struct {
		order  string
		credit int64
		want   int
	}{{"partial-debt-credit", 20, -30}, {"net-debt-credit", 40, 10}} {
		order := &model.TopUp{UserId: 1, TradeNo: tc.order, Amount: tc.credit, PaymentProvider: model.PaymentProviderEpay, PaymentMethod: "alipay", Status: common.TopUpStatusPending}
		require.NoError(t, model.DB.Create(order).Error)
		alreadyDone, err := model.RechargeEpay(tc.order, "alipay", "127.0.0.1")
		require.NoError(t, err)
		assert.False(t, alreadyDone)
		alreadyDone, err = model.RechargeEpay(tc.order, "alipay", "127.0.0.1")
		require.NoError(t, err)
		assert.True(t, alreadyDone)
		quota, err := model.GetUserQuota(1, true)
		require.NoError(t, err)
		assert.Equal(t, tc.want, quota)
		if tc.want < 0 {
			assert.NotNil(t, CheckGenerationDebt(info))
		} else {
			assert.Nil(t, CheckGenerationDebt(info))
		}
	}
	info.UserSetting.BillingPreference = "wallet_only"
	info.RequestId = "after-repayment"
	_, apiErr = NewBillingSession(c, info, 5)
	require.Nil(t, apiErr)
	quota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Equal(t, 5, quota)
}

func TestSubscriptionDebtStrictReserveAndPartialUsageFinalization(t *testing.T) {
	c, info := setupSubscriptionDebtBilling(t, 0, 1000)
	session, apiErr := NewBillingSession(c, info, 100)
	require.Nil(t, apiErr)
	require.Error(t, session.Reserve(200), "before-dispatch extra reservation cannot create debt")
	require.NoError(t, session.SettlePartial(80), "already reserved usage must not be debited twice")
	require.NoError(t, session.SettlePartial(200))
	require.NoError(t, session.SettlePartial(200))
	assert.False(t, session.NeedsRefund(), "delivered partial usage must not receive a full failure refund")
	require.NoError(t, session.Settle(200))
	quota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Equal(t, -50, quota)
	var token model.Token
	require.NoError(t, model.DB.First(&token, 1).Error)
	assert.Equal(t, 200, token.UsedQuota)
	assert.Equal(t, int64(50), info.SubscriptionWalletConsumed)
	assert.Equal(t, int64(50), info.SubscriptionPostDelta)
}

func TestSubscriptionDebtLegacySettlementUsesSameAllocation(t *testing.T) {
	_, info := setupSubscriptionDebtBilling(t, 0, 1000)
	info.SubscriptionId = 1
	info.BillingSource = BillingSourceSubscription
	require.NoError(t, PostConsumeQuota(info, 200, 0, false))
	quota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Equal(t, -50, quota)
	assert.Equal(t, int64(150), info.SubscriptionPostDelta)
	assert.Equal(t, int64(50), info.SubscriptionWalletConsumed)
	require.NoError(t, PostConsumeQuota(info, -70, 0, false))
	quota, err = model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Zero(t, quota)
	assert.Zero(t, info.SubscriptionWalletConsumed)
	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, 1).Error)
	assert.Equal(t, int64(980), sub.AmountUsed)
}

func TestSubscriptionDebtRealtimeUsageChargesOnceAcrossClose(t *testing.T) {
	c, info := setupSubscriptionDebtBilling(t, 0, 1000)
	oldModels, oldGroups := ratio_setting.ModelRatio2JSONString(), ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"debt-realtime":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"debt-test":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModels))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
	})
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 1, UpstreamModelName: "debt-realtime"}
	info.OriginModelName = "debt-realtime"
	info.UserGroup, info.UsingGroup = "debt-test", "debt-test"
	info.StartTime = time.Now()
	info.PriceData.ModelRatio = 1
	info.PriceData.GroupRatioInfo.GroupRatio = 1
	require.Nil(t, PreConsumeBilling(c, 100, info))
	usage := func(tokens int) *dto.RealtimeUsage {
		u := &dto.RealtimeUsage{InputTokens: tokens, TotalTokens: tokens}
		u.InputTokenDetails.TextTokens = tokens
		return u
	}
	require.NoError(t, PreWssConsumeQuota(c, info, usage(80)))
	assert.Nil(t, CheckGenerationDebt(info))
	require.NoError(t, PreWssConsumeQuota(c, info, usage(120)))
	assert.NotNil(t, CheckGenerationDebt(info))
	assert.Equal(t, 200, info.RealtimeConsumedQuota)
	PostWssConsumeQuota(c, info, info.OriginModelName, usage(200), nil)
	info.Billing.Refund(c)
	quota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Equal(t, -50, quota)
	var token model.Token
	require.NoError(t, model.DB.First(&token, 1).Error)
	assert.Equal(t, 200, token.UsedQuota)
	assert.Equal(t, int64(50), info.SubscriptionWalletConsumed)
	assert.False(t, info.Billing.(*BillingSession).NeedsRefund())
}

func TestSubscriptionDebtTaskReplayAndRefundUseStoredAllocation(t *testing.T) {
	c, info := setupSubscriptionDebtBilling(t, 0, 1000)
	session, apiErr := NewBillingSession(c, info, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(100))
	task := &model.Task{UserId: 1, TaskID: "debt-async", Quota: 100,
		PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: 1, TokenId: 1,
			// fork: the controller stamps the subscription's allowance generation at insert.
			SubscriptionAllowanceEpoch: 1, SubscriptionAllowanceAllocated: 100}}
	require.NoError(t, model.DB.Create(task).Error)
	stale := *task
	RecalculateTaskQuota(context.Background(), task, 200, common.NewMessage("completed usage"))
	RecalculateTaskQuota(context.Background(), &stale, 200, common.NewMessage("duplicate completion"))
	quota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Equal(t, -50, quota)
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, 200, stored.Quota)
	assert.Equal(t, int64(50), stored.PrivateData.SubscriptionWalletConsumed)
	var token model.Token
	require.NoError(t, model.DB.First(&token, 1).Error)
	assert.Equal(t, 200, token.UsedQuota)
	require.True(t, RefundTaskQuota(context.Background(), &stored, "task refund"))
	require.True(t, RefundTaskQuota(context.Background(), &stale, "duplicate refund"))
	quota, err = model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Zero(t, quota)
	require.NoError(t, model.DB.First(&token, 1).Error)
	assert.Zero(t, token.UsedQuota)
	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, 1).Error)
	assert.Equal(t, int64(850), sub.AmountUsed)
}
