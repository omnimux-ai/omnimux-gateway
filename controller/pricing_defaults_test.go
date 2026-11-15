package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterPricingDefaultsKeepsOnlyVisibleModelsWithGroups(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "gemini-3.8-flash", EnableGroup: []string{"default"}},
		{ModelName: "gpt-image-2.5", EnableGroup: []string{}},
		{ModelName: "seedance-2-5", EnableGroup: []string{"default"}},
	}
	defaults := map[string]string{
		"chat":           "gemini-3.8-flash",
		"text_to_image":  "gpt-image-2.5",
		"text_to_video":  "seedance-2-5",
		"text_to_music":  "suno",
		"speech_to_text": "whisper-1",
	}

	got := filterPricingDefaults(pricing, defaults)

	require.Equal(t, map[string]string{
		"chat":          "gemini-3.8-flash",
		"text_to_video": "seedance-2-5",
	}, got)
}

func TestFilterPricingDefaultsReturnsNilWhenNoRecommendationIsVisible(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "unrelated-model", EnableGroup: []string{"default"}},
	}
	defaults := map[string]string{
		"chat": "gemini-3.8-flash",
	}

	assert.Nil(t, filterPricingDefaults(pricing, defaults))
	assert.Nil(t, filterPricingDefaults(nil, defaults))
	assert.Nil(t, filterPricingDefaults(pricing, nil))
}

func TestPricingEndpointPublishesOnlyLiveExecutableContracts(t *testing.T) {
	withSelfUseModeDisabled(t)
	db := setupModelListControllerTestDB(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		model.InvalidatePricingCache()
	})

	require.NoError(t, db.Create(&model.Channel{
		Id:     801,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "catalog-contract-gate-key",
		Status: common.ChannelStatusEnabled,
		Name:   "catalog-contract-gate-channel",
		Group:  "default",
	}).Error)
	for _, modelName := range []string{"seedance-2-5", "kling-v3", "muse-image"} {
		require.NoError(t, db.Create(&model.Ability{
			Group:     "default",
			Model:     modelName,
			ChannelId: 801,
			Enabled:   true,
		}).Error)
	}

	model.InitChannelCache()
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	GetPricing(ctx)

	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ModelName string `json:"model_name"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	names := make([]string, 0, len(payload.Data))
	for _, row := range payload.Data {
		names = append(names, row.ModelName)
	}
	assert.Contains(t, names, "seedance-2-5")
	assert.NotContains(t, names, "kling-v3")
	assert.NotContains(t, names, "muse-image")
}
