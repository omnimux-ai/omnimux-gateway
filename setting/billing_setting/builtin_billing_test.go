package billing_setting_test

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPT6AstraBuiltinBilling(t *testing.T) {
	settings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	saved := *settings
	savedRatios, savedPrices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	savedOptions := common.OptionMap
	t.Cleanup(func() {
		*settings, common.OptionMap = saved, savedOptions
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})
	common.OptionMap = map[string]string{"billing_setting.billing_mode": `{}`, "billing_setting.billing_expr": `{}`}
	require.NoError(t, config.GlobalConfig.LoadFromDB(common.OptionMap))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode("gpt-6-astra"))
	expression, ok := billing_setting.GetBillingExpr("gpt-6-astra")
	require.True(t, ok)

	for _, tc := range []struct {
		name                           string
		input, output, cached, written int
		request                        string
		quota                          int
	}{
		{"standard", 1000, 100, 0, 0, `{}`, 7500},
		{"client flex cannot discount standard pricing", 1000, 100, 0, 0, `{"service_tier":"flex"}`, 7500},
		{"cache at context boundary", 272000, 1000, 200000, 20000, `{}`, 510000},
		{"whole request above boundary", 272001, 1000, 200000, 20000, `{}`, 1007510},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usage := &dto.Usage{
				PromptTokens: tc.input, CompletionTokens: tc.output,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: tc.cached, CacheWriteTokens: tc.written},
			}
			params := service.BuildTieredTokenParams(usage, false, billingexpr.UsedVars(expression))
			result, err := billingexpr.ComputeTieredQuotaWithRequest(&billingexpr.BillingSnapshot{
				ExprString: expression, GroupRatio: 1, QuotaPerUnit: 500000,
			}, params, billingexpr.RequestInput{Body: []byte(tc.request)})
			require.NoError(t, err)
			assert.Equal(t, tc.quota, result.ActualQuotaAfterGroup)
		})
	}

	t.Run("admin options expose defaults without persisting them", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		controller.GetOptions(ctx)
		var response struct {
			Success bool
			Data    []model.Option
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		require.True(t, response.Success)
		found := map[string]string{}
		for _, option := range response.Data {
			if _, ok := common.OptionMap[option.Key]; ok {
				assert.NotContains(t, found, option.Key)
				var values map[string]string
				require.NoError(t, common.UnmarshalJsonStr(option.Value, &values))
				found[option.Key] = values["gpt-6-astra"]
				assert.Equal(t, `{}`, common.OptionMap[option.Key])
			}
		}
		assert.Equal(t, map[string]string{"billing_setting.billing_mode": "tiered_expr", "billing_setting.billing_expr": expression}, found)
	})

	for _, tc := range []struct {
		name, mode, expr, ratios, prices, wantMode string
	}{
		{"custom expression overrides legacy price", "tiered_expr", "p * 7", `{"gpt-6-astra":8}`, `{}`, "tiered_expr"},
		{"explicit ratio mode", "ratio", "", `{}`, `{}`, "ratio"},
		{"existing free token price", "", "", `{"gpt-6-astra":0}`, `{}`, "ratio"},
		{"existing per-call price", "", "", `{}`, `{"gpt-6-astra":0.1}`, "ratio"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*settings = billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
			if tc.mode != "" {
				settings.BillingMode["gpt-6-astra"] = tc.mode
			}
			if tc.expr != "" {
				settings.BillingExpr["gpt-6-astra"] = tc.expr
			}
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(tc.ratios))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(tc.prices))
			assert.Equal(t, tc.wantMode, billing_setting.GetBillingMode("gpt-6-astra"))
			actual, ok := billing_setting.GetBillingExpr("gpt-6-astra")
			assert.Equal(t, tc.expr, actual)
			assert.Equal(t, tc.expr != "", ok)
		})
	}
}

func TestImageModelBuiltinPricesAndOverrides(t *testing.T) {
	settings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	saved := *settings
	savedRatios, savedPrices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*settings = saved
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})
	for _, name := range []string{"gpt-image-2", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare"} {
		t.Run(name, func(t *testing.T) {
			*settings = billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
			expression, ok := billing_setting.GetBillingExpr(name)
			require.True(t, ok)
			usage := &dto.Usage{PromptTokens: 1000, CompletionTokens: 100, PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 300, ImageTokens: 600, CachedTokensDetails: &dto.CachedTokenDetails{ImageTokens: common.GetPointer(200)},
			}}
			result, err := billingexpr.ComputeTieredQuota(&billingexpr.BillingSnapshot{ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), GroupRatio: 1, QuotaPerUnit: 500000},
				service.BuildTieredTokenParams(usage, false, billingexpr.UsedVars(expression)))
			require.NoError(t, err)
			assert.Equal(t, 4113, result.ActualQuotaAfterGroup)
			encoded, err := common.Marshal(map[string]float64{name: 0})
			require.NoError(t, err)
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
			assert.Equal(t, "ratio", billing_setting.GetBillingMode(name))
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(encoded)))
			assert.Equal(t, "ratio", billing_setting.GetBillingMode(name))
			settings.BillingMode[name] = "tiered_expr"
			settings.BillingExpr[name] = `tier("custom", p * 7)`
			actual, ok := billing_setting.GetBillingExpr(name)
			require.True(t, ok)
			assert.Equal(t, settings.BillingExpr[name], actual)
		})
	}
}

func TestVolcengineTTS3BuiltinBillingAndOverrides(t *testing.T) {
	settings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	saved := *settings
	savedRatios, savedPrices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*settings = saved
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})

	for _, name := range []string{"seed-tts-3.0", "doubao-seed-tts-3.0"} {
		t.Run(name, func(t *testing.T) {
			*settings = billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))

			assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode(name))
			expression, ok := billing_setting.GetBillingExpr(name)
			require.True(t, ok)
			assert.Equal(t, `tier("standard", p * 21)`, expression)

			// 10,000 billable chars at $21 / 1M chars = $0.21 -> 105,000 quota at QuotaPerUnit=500000.
			usage := &dto.Usage{PromptTokens: 10000, TotalTokens: 10000}
			result, err := billingexpr.ComputeTieredQuota(
				&billingexpr.BillingSnapshot{
					ExprString:   expression,
					ExprHash:     billingexpr.ExprHashString(expression),
					GroupRatio:   1,
					QuotaPerUnit: 500000,
				},
				service.BuildTieredTokenParams(usage, false, billingexpr.UsedVars(expression)),
			)
			require.NoError(t, err)
			assert.Equal(t, 105000, result.ActualQuotaAfterGroup)

			// Admin ratio / price overrides take precedence over built-in defaults.
			encoded, err := common.Marshal(map[string]float64{name: 10.5})
			require.NoError(t, err)
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
			assert.Equal(t, billing_setting.BillingModeRatio, billing_setting.GetBillingMode(name))

			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(encoded)))
			assert.Equal(t, billing_setting.BillingModeRatio, billing_setting.GetBillingMode(name))

			// Custom admin expression overrides built-in expression.
			settings.BillingMode[name] = billing_setting.BillingModeTieredExpr
			settings.BillingExpr[name] = `tier("custom", p * 30)`
			actual, ok := billing_setting.GetBillingExpr(name)
			require.True(t, ok)
			assert.Equal(t, `tier("custom", p * 30)`, actual)
		})
	}
}

func TestGemini4BuiltinBillingAndOverrides(t *testing.T) {
	settings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	saved := *settings
	savedRatios, savedPrices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*settings = saved
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})

	for _, tc := range []struct {
		name     string
		wantExpr string
		quota    int
	}{
		{"gemini-4-hydrogen", `tier("standard", p * 1.5 + c * 7.5 + cr * 0.075)`, 9825},
		{"gemini-4-neon", `tier("standard", p * 3 + c * 15 + cr * 0.15)`, 19650},
		{"gemini-4-carbon", `tier("standard", p * 4 + c * 20 + cr * 0.2)`, 26200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*settings = billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))

			assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode(tc.name))
			expression, ok := billing_setting.GetBillingExpr(tc.name)
			require.True(t, ok)
			assert.Equal(t, tc.wantExpr, expression)

			usage := &dto.Usage{
				PromptTokens:     10000,
				CompletionTokens: 1000,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 2000,
				},
			}
			result, err := billingexpr.ComputeTieredQuota(
				&billingexpr.BillingSnapshot{
					ExprString:   expression,
					ExprHash:     billingexpr.ExprHashString(expression),
					GroupRatio:   1,
					QuotaPerUnit: 500000,
				},
				service.BuildTieredTokenParams(usage, false, billingexpr.UsedVars(expression)),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.quota, result.ActualQuotaAfterGroup)

			encoded, err := common.Marshal(map[string]float64{tc.name: 1})
			require.NoError(t, err)
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
			assert.Equal(t, billing_setting.BillingModeRatio, billing_setting.GetBillingMode(tc.name))

			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(encoded)))
			assert.Equal(t, billing_setting.BillingModeRatio, billing_setting.GetBillingMode(tc.name))

			settings.BillingMode[tc.name] = billing_setting.BillingModeTieredExpr
			settings.BillingExpr[tc.name] = `tier("custom", p * 9)`
			actual, ok := billing_setting.GetBillingExpr(tc.name)
			require.True(t, ok)
			assert.Equal(t, `tier("custom", p * 9)`, actual)
		})
	}
}

func TestTypeSafeJevBuiltinBillingAndOverrides(t *testing.T) {
	settings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	saved := *settings
	savedRatios, savedPrices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*settings = saved
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})

	schema := map[string]jsplugin.UsageFieldSchema{
		"input_tokens": {Type: "number", Unit: "token"},
	}
	const wantExpr = `tier("base", u("input_tokens") * 0.042 / 1000000)`
	require.NoError(t, billing_setting.SmokeTestTaskExpr(wantExpr, schema))

	for _, name := range []string{"jev-1.14.0", "jev-1.13.0", "jev-latest", "jev-preview"} {
		t.Run(name, func(t *testing.T) {
			*settings = billing_setting.BillingSetting{
				BillingMode:       map[string]string{},
				BillingExpr:       map[string]string{},
				PluginBillingExpr: map[string]string{},
			}
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))

			assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode(name))
			expr, ok := billing_setting.ResolveTaskBillingExpr("typesafe", name, name)
			require.True(t, ok)
			assert.Equal(t, wantExpr, expr)

			for _, tc := range []struct {
				inputTokens int
				wantQuota   int
			}{
				{100, 2},
				{100000, 2100},
				{65792, 1382},
				{262400, 5510},
			} {
				res, err := billingexpr.ComputeTieredQuotaWithRequest(&billingexpr.BillingSnapshot{
					ExprString:       expr,
					ExprHash:         billingexpr.ExprHashString(expr),
					GroupRatio:       1,
					QuotaPerUnit:     500000,
					ExprVersion:      1,
					TaskUsageBilling: true,
				}, billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: map[string]any{"input_tokens": float64(tc.inputTokens)}})
				require.NoError(t, err)
				assert.Equal(t, tc.wantQuota, res.ActualQuotaAfterGroup)
			}

			settings.PluginBillingExpr["typesafe::"+name] = `tier("custom", u("input_tokens") * 0.1 / 1000000)`
			override, ok := billing_setting.ResolveTaskBillingExpr("typesafe", name, name)
			require.True(t, ok)
			assert.Equal(t, `tier("custom", u("input_tokens") * 0.1 / 1000000)`, override)
		})
	}

	t.Run("legacy jev alias falls back to jev-latest built-in expression", func(t *testing.T) {
		*settings = billing_setting.BillingSetting{
			BillingMode:       map[string]string{},
			BillingExpr:       map[string]string{},
			PluginBillingExpr: map[string]string{},
		}
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"jev": 0.021}`))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))

		expr, ok := billing_setting.ResolveTaskBillingExpr("typesafe", "jev", "jev-latest")
		require.True(t, ok)
		assert.Equal(t, wantExpr, expr)

		settings.BillingMode["jev"] = billing_setting.BillingModeTieredExpr
		settings.BillingExpr["jev"] = `tier("alias", u("input_tokens") * 0.084 / 1000000)`
		custom, ok := billing_setting.ResolveTaskBillingExpr("typesafe", "jev", "jev-latest")
		require.True(t, ok)
		assert.Equal(t, `tier("alias", u("input_tokens") * 0.084 / 1000000)`, custom)
	})
}
