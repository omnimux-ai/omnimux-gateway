package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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
	for _, modelName := range []string{"seedance-2-5", "kling-v3", "muse-chat"} {
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
	assert.NotContains(t, names, "muse-chat")
}

func TestVideoCapabilitiesCatalogIsScopedAndMatchesModelContract(t *testing.T) {
	withSelfUseModeDisabled(t)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousDBType, previousLogDBType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis := common.RedisEnabled
	previousCache := common.MemoryCacheEnabled
	previousUsable := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousDBType, previousLogDBType)
		common.RedisEnabled = previousRedis
		common.MemoryCacheEnabled = previousCache
		model.InvalidatePricingCache()
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","unverified":"Unverified","no-ratio":"No ratio","disabled":"Disabled"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"unverified":1,"members":2,"disabled":1}`))
	db := setupModelListControllerTestDB(t)
	const modelID = "seedance-2-5"
	const userID = 79
	const baseURL = "https://catalog-upstream.invalid"
	require.NoError(t, db.Create(&model.User{Id: userID, Username: "video-catalog-member", Group: "members"}).Error)
	var rulesJSON []string
	for _, line := range []struct {
		id         int
		group      string
		status     int
		maxImages  int
		resolution string
	}{
		{81, "default", common.ChannelStatusEnabled, 2, "720p"},
		{82, "default", common.ChannelStatusEnabled, 8, "480p"},
		{83, "default", common.ChannelStatusEnabled, 2, "720p"},
		{84, "members", common.ChannelStatusEnabled, 10, "1080p"},
		{85, "no-ratio", common.ChannelStatusEnabled, 2, "720p"},
		{86, "disabled", common.ChannelStatusManuallyDisabled, 2, "720p"},
		{87, "unverified", common.ChannelStatusEnabled, 0, ""},
	} {
		require.NoError(t, db.Create(&model.Channel{
			Id: line.id, Type: constant.ChannelTypeApiMart, Status: line.status,
			Key: "catalog-fixture", Group: line.group, Models: modelID, BaseURL: common.GetPointer(baseURL),
		}).Error)
		require.NoError(t, db.Create(&model.Ability{
			ChannelId: line.id, Group: line.group, Model: modelID, Enabled: true,
		}).Error)
		if line.maxImages > 0 {
			rulesJSON = append(rulesJSON, fmt.Sprintf(`{"channel_id":%d,"channel_type":%d,"profile":"apimart_video","upstream_model":%q,"base_url":%q,"evidence":"fixture evidence","operations":[{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":%d}],"parameters":[{"name":"resolution","enum":[%q]}]}]}`,
				line.id, constant.ChannelTypeApiMart, modelID, baseURL, line.maxImages, line.resolution))
		}
	}
	rules, err := modelcontracts.ReadVideoCapabilities(strings.NewReader(`{"models":{"muse-video":[],"seedance-2-5":[` + strings.Join(rulesJSON, ",") + `]}}`))
	require.NoError(t, err)
	oldRules := modelcontracts.SetVideoCapabilities(rules)
	t.Cleanup(func() { modelcontracts.SetVideoCapabilities(oldRules) })

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if c.GetHeader("Catalog-Fixture-Member") == "1" {
			c.Set("id", userID)
		}
	})
	engine.GET("/api/pricing", GetPricing)
	engine.GET("/api/model_contract/:model", GetModelContract)
	read := func(t *testing.T, path string, authenticated bool, target any) {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if authenticated {
			request.Header.Set("Catalog-Fixture-Member", "1")
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		if path != "/api/model_contract/wan-3.0" {
			assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
		}
		assert.NotContains(t, recorder.Body.String(), "fixture evidence")
		assert.NotContains(t, recorder.Body.String(), `"channel_id"`)
		assert.NotContains(t, recorder.Body.String(), `"upstream_model"`)
		assert.NotContains(t, recorder.Body.String(), baseURL)
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), target))
	}
	type contractResponse struct {
		Data struct {
			EffectiveVideo *model.EffectiveVideo `json:"effective_video"`
		} `json:"data"`
	}
	var uncached *model.EffectiveVideo
	for _, useCache := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory cache %t", useCache), func(t *testing.T) {
			common.MemoryCacheEnabled = useCache
			model.InitChannelCache()
			var empty, unmanaged contractResponse
			read(t, "/api/model_contract/muse-video", false, &empty)
			require.NotNil(t, empty.Data.EffectiveVideo)
			assert.Equal(t, map[string][]modelcontracts.Operation{}, empty.Data.EffectiveVideo.Groups)
			read(t, "/api/model_contract/wan-3.0", false, &unmanaged)
			assert.Nil(t, unmanaged.Data.EffectiveVideo)
			var anonymousVersion string
			for _, authenticated := range []bool{false, true} {
				var pricing struct {
					Data []model.Pricing `json:"data"`
				}
				var contract contractResponse
				read(t, "/api/pricing", authenticated, &pricing)
				read(t, "/api/model_contract/"+modelID, authenticated, &contract)
				require.Len(t, pricing.Data, 1)
				view := pricing.Data[0].EffectiveVideo
				require.NotNil(t, view)
				assert.Equal(t, view, contract.Data.EffectiveVideo)
				assert.NotEmpty(t, view.Version)
				assert.NotContains(t, view.Groups, "disabled")
				assert.NotContains(t, view.Groups, "no-ratio")
				assert.Equal(t, []modelcontracts.Operation{}, view.Groups["unverified"])
				if authenticated {
					require.Len(t, view.Groups["members"], 1)
					assert.NotEqual(t, anonymousVersion, view.Version)
				} else {
					anonymousVersion = view.Version
					assert.NotContains(t, view.Groups, "members")
					if useCache {
						assert.Equal(t, uncached, view)
					} else {
						uncached = view
					}
				}
				variants := view.Groups["default"]
				require.Len(t, variants, 2)
				limits := make(map[string]int)
				for _, variant := range variants {
					require.Len(t, variant.Inputs, 2)
					require.NotNil(t, variant.Inputs[1].Max)
					require.Len(t, variant.Parameters, 1)
					require.Len(t, variant.Parameters[0].Enum, 1)
					resolution, ok := variant.Parameters[0].Enum[0].(string)
					require.True(t, ok)
					limits[resolution] = *variant.Inputs[1].Max
				}
				assert.Equal(t, map[string]int{"720p": 2, "480p": 8}, limits)
			}
		})
	}
}
