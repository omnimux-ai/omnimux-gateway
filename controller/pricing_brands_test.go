package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterPricingBrandsIncludesOnlyVisibleModels(t *testing.T) {
	brands := map[string]modelcontracts.BrandMeta{
		"openai":    {ID: "openai", Name: "OpenAI"},
		"bytedance": {ID: "bytedance", Name: "ByteDance"},
		"hidden":    {ID: "hidden", Name: "Hidden Brand", IconPath: "M0 0h24v24H0z"},
	}
	pricing := []model.Pricing{
		{ModelName: "gpt-image-2.5", BrandID: "openai", BrandName: "OpenAI"},
		{ModelName: "seedance-2-5", Contract: &modelcontracts.Contract{BrandID: "bytedance", BrandName: "ByteDance"}},
		{ModelName: "custom-model"},
	}

	got := filterPricingBrands(pricing, brands)
	require.Equal(t, map[string]modelcontracts.BrandMeta{
		"openai":    {ID: "openai", Name: "OpenAI"},
		"bytedance": {ID: "bytedance", Name: "ByteDance"},
	}, got)
	assert.NotContains(t, got, "hidden")
}

func TestGetPricingPublishesCanonicalBrandMetadataAndPreservesVendors(t *testing.T) {
	withSelfUseModeDisabled(t)
	db := setupModelListControllerTestDB(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		model.InvalidatePricingCache()
	})

	require.NoError(t, db.Create(&model.Channel{
		Id:     811,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "pricing-brand-test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "pricing-brand-test-channel",
		Group:  "default",
	}).Error)
	for _, modelName := range []string{"seedance-2-5", "gpt-image-2.5"} {
		require.NoError(t, db.Create(&model.Ability{
			Group:     "default",
			Model:     modelName,
			ChannelId: 811,
			Enabled:   true,
		}).Error)
	}

	model.InitChannelCache()
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Success bool                              `json:"success"`
		Vendors []model.PricingVendor             `json:"vendors"`
		Brands  map[string]modelcontracts.BrandMeta `json:"brands"`
		Data    []struct {
			ModelName string `json:"model_name"`
			BrandID   string `json:"brand_id"`
			BrandName string `json:"brand_name"`
			Contract  *struct {
				BrandID         string `json:"brand_id"`
				BrandName       string `json:"brand_name"`
				ContractVersion string `json:"contractVersion"`
			} `json:"contract"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	byModel := make(map[string]struct {
		brandID   string
		brandName string
		contract  *struct {
			BrandID         string `json:"brand_id"`
			BrandName       string `json:"brand_name"`
			ContractVersion string `json:"contractVersion"`
		}
	})
	for _, row := range payload.Data {
		byModel[row.ModelName] = struct {
			brandID   string
			brandName string
			contract  *struct {
				BrandID         string `json:"brand_id"`
				BrandName       string `json:"brand_name"`
				ContractVersion string `json:"contractVersion"`
			}
		}{brandID: row.BrandID, brandName: row.BrandName, contract: row.Contract}
	}

	require.Contains(t, byModel, "seedance-2-5")
	assert.Equal(t, "bytedance", byModel["seedance-2-5"].brandID)
	assert.Equal(t, "ByteDance", byModel["seedance-2-5"].brandName)
	require.NotNil(t, byModel["seedance-2-5"].contract)
	assert.Equal(t, "bytedance", byModel["seedance-2-5"].contract.BrandID)
	assert.Equal(t, "ByteDance", byModel["seedance-2-5"].contract.BrandName)

	require.Contains(t, byModel, "gpt-image-2.5")
	assert.Equal(t, "openai", byModel["gpt-image-2.5"].brandID)
	assert.Equal(t, "OpenAI", byModel["gpt-image-2.5"].brandName)

	require.Contains(t, payload.Brands, "bytedance")
	require.Contains(t, payload.Brands, "openai")
	assert.Equal(t, modelcontracts.BrandMeta{ID: "bytedance", Name: "ByteDance"}, payload.Brands["bytedance"])
	assert.Equal(t, modelcontracts.BrandMeta{ID: "openai", Name: "OpenAI"}, payload.Brands["openai"])
	assert.NotContains(t, payload.Brands, "minimax", "delisted or unserved brands must not leak into top-level brands map")
}
