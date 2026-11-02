package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sunburstPricingTestModel = "gpt-image-2.5-sunburst"

func setupSunburstPricing(t *testing.T, expression string, legacyPrice bool) {
	t.Helper()
	resetPricingEndpointTestTables(t)
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	prices, err := common.Marshal(ratio_setting.GetModelPriceCopy())
	require.NoError(t, err)
	ratios, err := common.Marshal(ratio_setting.GetModelRatioCopy())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(prices)))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(ratios)))
		InvalidatePricingCache()
	})
	priceMap := ratio_setting.GetModelPriceCopy()
	delete(priceMap, sunburstPricingTestModel)
	if legacyPrice {
		priceMap[sunburstPricingTestModel] = 999
	}
	priceJSON, err := common.Marshal(priceMap)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(priceJSON)))
	ratioMap := ratio_setting.GetModelRatioCopy()
	delete(ratioMap, sunburstPricingTestModel)
	ratioJSON, err := common.Marshal(ratioMap)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(ratioJSON)))
	exprJSON, err := common.Marshal(map[string]string{sunburstPricingTestModel: expression})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"gpt-image-2.5-sunburst":"tiered_expr"}`,
		"billing_setting.billing_expr": string(exprJSON),
	}))
	insertPricingEndpointChannel(t, 991, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 991, sunburstPricingTestModel)
	InitChannelCache()
}

func TestSunburstPricingProjectsNativeFixedImageExpressionWithoutPersistentPrices(t *testing.T) {
	const expression = `tier("image", fixed(0.013072)) * image_count`
	setupSunburstPricing(t, expression, false)
	pricesBefore := ratio_setting.GetModelPriceCopy()
	ratiosBefore := ratio_setting.GetModelRatioCopy()
	pricing := pricingByModel(GetPricing())
	require.Contains(t, pricing, sunburstPricingTestModel)
	row := pricing[sunburstPricingTestModel]
	assert.Equal(t, 1, row.QuotaType)
	assert.Equal(t, 0.013072, row.ModelPrice)
	assert.Zero(t, row.ModelRatio)
	assert.Equal(t, "tiered_expr", row.BillingMode)
	assert.Equal(t, expression, row.BillingExpr)
	assert.Equal(t, pricesBefore, ratio_setting.GetModelPriceCopy())
	assert.Equal(t, ratiosBefore, ratio_setting.GetModelRatioCopy())
	assert.False(t, ratio_setting.HasConfiguredModelRatio(sunburstPricingTestModel))
	_, configured := ratio_setting.GetModelPrice(sunburstPricingTestModel, false)
	assert.False(t, configured)

	for _, test := range []struct {
		ratio float64
		quota int
	}{
		{1, 6536}, {0.9, 5882}, {0.8, 5229}, {0.112455, 735},
	} {
		native, err := billingexpr.ComputeTieredQuota(&billingexpr.BillingSnapshot{
			ExprString: expression, ExprHash: billingexpr.ExprHashString(expression),
			ExprVersion: 1, QuotaPerUnit: common.QuotaPerUnit, GroupRatio: test.ratio,
		}, billingexpr.TokenParams{})
		require.NoError(t, err)
		assert.Nil(t, native.Clamp)
		assert.Equal(t, test.quota, native.ActualQuotaAfterGroup)
		projected, clamp := common.QuotaRoundChecked(row.ModelPrice * common.QuotaPerUnit * test.ratio)
		assert.Nil(t, clamp)
		assert.Equal(t, native.ActualQuotaAfterGroup, projected)
	}
}

func TestSunburstPricingExpressionOverridesStaleLegacyPriceWithoutWritingIt(t *testing.T) {
	setupSunburstPricing(t, `v1:tier("image", fixed(0)) * image_count`, true)
	row := pricingByModel(GetPricing())[sunburstPricingTestModel]
	assert.Equal(t, 1, row.QuotaType)
	assert.Zero(t, row.ModelPrice)
	price, configured := ratio_setting.GetModelPrice(sunburstPricingTestModel, false)
	assert.True(t, configured)
	assert.Equal(t, 999.0, price)
}

func TestSunburstPricingRejectsNoncanonicalOrMissingFixedImageQuotes(t *testing.T) {
	for _, expression := range []string{
		"", `tier("image", ???)`, `tier("image", fixed(0.01))`,
		`tier("tokens", p * 2)`, `u("images") * 0.01`,
		`tier("image", fixed(0.01)) * image_count * image_count`,
		`true ? tier("image", fixed(0.01)) * image_count : tier("image", fixed(0.02)) * image_count`,
		`tier("image", fixed(0.01)) * image_count * (param("quality") == "high" ? 2 : 1)`,
		`tier("request", fixed(0.01)) * image_count`,
		`tier("image", fixed(-0.01)) * image_count`,
		`tier("image", fixed(1e309)) * image_count`,
		`tier("image", fixed(5000)) * image_count`,
	} {
		t.Run(expression, func(t *testing.T) {
			setupSunburstPricing(t, expression, true)
			assert.NotContains(t, pricingByModel(GetPricing()), sunburstPricingTestModel)
		})
	}
}

func TestSunburstPricingProjectionDoesNotHideOtherModels(t *testing.T) {
	setupSunburstPricing(t, `tier("image", p * 2)`, true)
	insertPricingEndpointAbility(t, 991, "unrelated-pricing-model")
	InitChannelCache()
	pricing := pricingByModel(GetPricing())
	assert.NotContains(t, pricing, sunburstPricingTestModel)
	require.Contains(t, pricing, "unrelated-pricing-model")
	assert.Equal(t, 0, pricing["unrelated-pricing-model"].QuotaType)
	assert.Equal(t, 37.5, pricing["unrelated-pricing-model"].ModelRatio)
}
