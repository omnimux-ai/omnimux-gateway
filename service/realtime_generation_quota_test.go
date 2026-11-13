package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtimeGenerationQuotaRequiresOriginalFiniteAllowance(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "missing"},
		{name: "invalid type", value: "1000"},
		{name: "zero", value: 0},
		{name: "negative", value: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, info := setupSubscriptionDebtBilling(t, 0, 10000)
			if tc.value != nil {
				ctx.Set("token_quota", tc.value)
			}
			apiErr := CheckRealtimeGenerationQuota(ctx, info)
			require.NotNil(t, apiErr)
			assert.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
			assert.True(t, types.IsSkipRetryError(apiErr))
		})
	}
}

func TestRealtimeGenerationQuotaPreservesNativeExemptions(t *testing.T) {
	setupSubscriptionDebtBilling(t, 0, 10000)
	for _, info := range []*relaycommon.RelayInfo{
		{IsChannelTest: true},
		{UserId: 1, IsPlayground: true},
		{UserId: 1, TokenUnlimited: true},
	} {
		assert.Nil(t, CheckRealtimeGenerationQuota(nil, info), "native exempt path must not need a token identity or original allowance")
	}
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = 1").Update("quota", -1).Error)
	apiErr := CheckRealtimeGenerationQuota(nil, &relaycommon.RelayInfo{UserId: 1, TokenUnlimited: true})
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
}

func TestRealtimeGenerationQuotaUsesBothNativeTokenSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		durable, cached, used int
	}{
		{name: "stale positive cache", durable: 0, cached: 1000},
		{name: "pending cache reservation", durable: 1000, cached: 0},
		{name: "both snapshots await own debit", durable: 1000, cached: 1000, used: 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, info := setupSubscriptionDebtBilling(t, 0, 10000)
			ctx.Set("token_quota", 1000)
			info.RealtimeConsumedQuota = tc.used
			server := miniredis.RunT(t)
			previousRDB, previousRedis := common.RDB, common.RedisEnabled
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			common.RDB, common.RedisEnabled = client, true
			t.Cleanup(func() {
				require.NoError(t, client.Close())
				common.RDB, common.RedisEnabled = previousRDB, previousRedis
			})
			token, err := model.GetTokenById(info.TokenId)
			require.NoError(t, err)
			require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", info.TokenId).Update("remain_quota", tc.durable).Error)
			token.RemainQuota = tc.cached
			require.NoError(t, common.RedisHSetObj("token:"+common.GenerateHMAC(info.TokenKey), token, 0))
			apiErr := CheckRealtimeGenerationQuota(ctx, info)
			require.NotNil(t, apiErr)
			assert.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
		})
	}
}

func TestRealtimeGenerationQuotaLimitIncreaseNeedsNewConnection(t *testing.T) {
	ctx, info := setupSubscriptionDebtBilling(t, 0, 10000)
	ctx.Set("token_quota", 1000)
	info.RealtimeConsumedQuota = 1000
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", info.TokenId).Update("remain_quota", 2000).Error)
	apiErr := CheckRealtimeGenerationQuota(ctx, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
	// A newly authenticated connection receives the new allowance snapshot.
	ctx.Set("token_quota", 2000)
	info.RealtimeConsumedQuota = 0
	assert.Nil(t, CheckRealtimeGenerationQuota(ctx, info))
}
