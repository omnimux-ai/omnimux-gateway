package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionDebtRejectsFreeGenerationBeforePriceOrUpstream(t *testing.T) {
	db := setupRelayChannelDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "free-debt", Quota: -1}).Error)
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	oldGroups := ratio_setting.GroupRatio2JSONString()
	oldModels := ratio_setting.ModelRatio2JSONString()
	oldFree := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"debt-free":0}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o":1}`))
	t.Cleanup(func() {
		common.RedisEnabled = oldRedis
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = oldFree
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModels))
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	info := &relaycommon.RelayInfo{UserId: 1, OriginModelName: "gpt-4o", UsingGroup: "debt-free", UserGroup: "debt-free", ChannelMeta: &relaycommon.ChannelMeta{}, UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
	apiErr := PrepareRequestBilling(c, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	assert.Nil(t, info.Billing)
	require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 0).Error)
	require.Nil(t, PrepareRequestBilling(c, info))
	assert.True(t, info.PriceData.FreeModel, "legitimate zero-debt free generation remains available")
	assert.Nil(t, info.Billing)
}

func TestSubscriptionDebtRejectsTaskAndLegacyActionBeforeDispatch(t *testing.T) {
	db := setupRelayChannelDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "task-debt", Quota: -1}).Error)
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	info := &relaycommon.RelayInfo{UserId: 1}
	info.PriceData.FreeModel = true
	_, taskErr := RelayTaskSubmit(c, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, string(types.ErrorCodeInsufficientUserQuota), taskErr.Code)
	assert.Nil(t, info.Billing)
	for _, call := range []func(*gin.Context, *relaycommon.RelayInfo) *appdto.MidjourneyResponse{RelayMidjourneySubmit, RelaySwapFace} {
		response := call(c, info)
		require.NotNil(t, response)
		assert.Contains(t, response.Description, "outstanding debt")
	}
}
