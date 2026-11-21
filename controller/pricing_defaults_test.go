package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/service"
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

func TestPricingGroupDefaultFallback(t *testing.T) {
	order := []string{"fixture-bronze", "default", "fixture-lime"}
	for _, tc := range []struct {
		name, configured, caller, want string
		usable                         map[string]string
	}{
		{"caller before default", "", "fixture-lime", "fixture-lime", map[string]string{"default": "Default", "fixture-lime": "Lime"}},
		{"default before first ordered", "", "fixture-absent", "default", map[string]string{"fixture-bronze": "Bronze", "default": "Default"}},
		{"first usable ordered", "", "", "fixture-lime", map[string]string{"fixture-lime": "Lime"}},
		{"no usable group", "", "", "", map[string]string{"fixture-absent": "Absent"}},
		{"configured but not enabled", "fixture-absent", "fixture-lime", "", map[string]string{"fixture-absent": "Absent", "fixture-lime": "Lime"}},
		{"configured but forbidden", "fixture-bronze", "fixture-lime", "", map[string]string{"fixture-lime": "Lime"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, pricingDefaultGroup(tc.configured, tc.caller, order, order, tc.usable))
		})
	}
}

func TestPricingGroupsDatabaseMatrix(t *testing.T) {
	require.NoError(t, i18n.Init())
	withSelfUseModeDisabled(t)
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to run this database")
			}
			testPricingGroups(t, dialect.kind, os.Getenv(dialect.env))
		})
	}
}

func testPricingGroups(t *testing.T, kind, dsn string) {
	t.Helper()
	db := modelManagementDB(t, kind, dsn)
	require.NoError(t, db.AutoMigrate(&model.UserSession{}))
	oldUsable := setting.UserUsableGroups2JSONString()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"fixture-lime":"Lime"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"fixture-lime":1.17,"fixture-bronze":2.39}`))

	const seedance = "seedance-2-5"
	const image = "gpt-image-2.5"
	const wan = "wan-3.0"
	const minimax = "minimax-h3-video"
	const lime = "fixture-lime"
	const bronze = "fixture-bronze"
	both := []string{lime, bronze}
	expectedOrders := map[string][]string{
		seedance: {bronze, lime}, image: {lime}, wan: both, minimax: {bronze},
	}
	for _, group := range both {
		var modelNames []string
		for name, order := range expectedOrders {
			if common.StringsContains(order, group) {
				modelNames = append(modelNames, name)
			}
		}
		channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: group, Key: "fixture", Models: strings.Join(modelNames, ",")}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	model.InvalidatePricingCache()
	legacyGroups := make(map[string][]string)
	for _, row := range model.GetCatalogPricing() {
		legacyGroups[row.ModelName] = row.EnableGroup
	}
	require.Len(t, legacyGroups, len(expectedOrders))

	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.GET("/api/pricing", middleware.HeaderNavModuleAuth("pricing"), GetPricing)
	api.GET("/api/option/", middleware.RootAuth(), GetOptions)
	api.PUT("/api/option/", middleware.RootAuth(), UpdateOption)
	credentials := make(map[int]string)
	for _, role := range []int{common.RoleCommonUser, common.RoleRootUser} {
		user := model.User{Username: fmt.Sprintf("pricing-fixture-%d", role), AffCode: fmt.Sprintf("pg%d", role), Role: role, Status: common.UserStatusEnabled, Group: bronze, AuthVersion: 1}
		require.NoError(t, db.Create(&user).Error)
		bundle, err := service.CreateLoginSession(user.Id, "fixture", "127.0.0.1", "pricing fixture")
		require.NoError(t, err)
		credentials[role] = bundle.AccessToken
	}
	call := func(method, path string, role int, body io.Reader) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, body)
		if bearer := credentials[role]; bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	const optionPath = "/api/option/"
	const checkPath = optionPath + "?key=" + model.PricingGroupConfigKey
	root := common.RoleRootUser
	member := common.RoleCommonUser
	assert.Equal(t, http.StatusUnauthorized, call(http.MethodGet, checkPath, 0, nil).Code)
	assert.Equal(t, http.StatusForbidden, call(http.MethodGet, checkPath, member, nil).Code)
	assert.Equal(t, http.StatusForbidden, call(http.MethodPut, optionPath, member, strings.NewReader(`{}`)).Code)

	type optionView struct {
		Success   bool                            `json:"success"`
		Data      []model.Option                  `json:"data"`
		Issues    []model.PricingGroupConfigIssue `json:"pricing_group_config_issues"`
		Fallbacks uint64                          `json:"pricing_group_fallback_rows"`
		Failures  uint64                          `json:"pricing_group_read_failures"`
	}
	check := func() (optionView, model.PricingGroupConfig) {
		var view optionView
		var config model.PricingGroupConfig
		response := call(http.MethodGet, checkPath, root, nil)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &view))
		require.True(t, view.Success, response.Body.String())
		require.Len(t, view.Data, 1)
		require.NoError(t, common.UnmarshalJsonStr(view.Data[0].Value, &config))
		return view, config
	}
	putOption := func(key string, config model.PricingGroupConfig, status int, success bool) {
		value, err := common.Marshal(config)
		require.NoError(t, err)
		body, err := common.Marshal(OptionUpdateRequest{Key: key, Value: string(value)})
		require.NoError(t, err)
		response := call(http.MethodPut, optionPath, root, strings.NewReader(string(body)))
		require.Equal(t, status, response.Code, response.Body.String())
		var result struct{ Success bool }
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
		require.Equal(t, success, result.Success, response.Body.String())
	}
	put := func(config model.PricingGroupConfig, status int, success bool) {
		putOption(model.PricingGroupConfigKey, config, status, success)
	}
	verifyOptionAliases := func(config model.PricingGroupConfig) {
		if kind != "mysql" {
			return
		}
		encoded, err := common.Marshal(config)
		require.NoError(t, err)
		// The disposable database uses a collation where all spellings below
		// collide with the canonical key, including before the option exists.
		for _, alias := range []string{"modelpricinggroups", "ModelPricingGroups ", "MódelPricingGroups"} {
			putOption(alias, config, http.StatusOK, false)
			require.Error(t, model.UpdateOptionsBulk(map[string]string{alias: string(encoded), "FixtureOptionA": "unexpected"}))
		}
		var rows int64
		require.NoError(t, db.Model(&model.Option{}).Where(&model.Option{Key: "FixtureOptionA"}).Count(&rows).Error)
		assert.Zero(t, rows)
	}
	readPricing := func(role int, compareLegacy bool) map[string]pricingResponseRow {
		response := call(http.MethodGet, "/api/pricing", role, nil)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		var payload struct {
			Success        bool
			Data           []pricingResponseRow
			Usable         map[string]string `json:"usable_group"`
			PricingVersion string            `json:"pricing_version"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
		require.True(t, payload.Success)
		assert.NotEmpty(t, payload.PricingVersion)
		assert.NotEqual(t, "a42d372ccf0b5dd13ecf71203521f9d2", payload.PricingVersion)
		rows := make(map[string]pricingResponseRow)
		for _, row := range payload.Data {
			if compareLegacy {
				assert.Equal(t, legacyGroups[row.ModelName], row.EnableGroup)
			}
			assert.ElementsMatch(t, row.EnableGroup, row.GroupOrder)
			if row.DefaultGroup != "" {
				assert.Contains(t, row.EnableGroup, row.DefaultGroup)
				assert.Contains(t, payload.Usable, row.DefaultGroup)
			}
			rows[row.ModelName] = row
		}
		return rows
	}

	initial, draft := check()
	require.Len(t, initial.Issues, 4)
	assert.Zero(t, draft.Version)
	for name, order := range expectedOrders {
		if name != minimax {
			draft.Models[name] = model.PricingGroupEntry{DefaultGroup: lime, GroupOrder: order}
		}
	}
	verifyOptionAliases(draft)
	_, stillEmpty := check()
	assert.Zero(t, stillEmpty.Version)
	assert.Empty(t, stillEmpty.Models)
	put(draft, http.StatusOK, true)
	coverage, saved := check()
	assert.EqualValues(t, 1, saved.Version)
	assert.Equal(t, []model.PricingGroupConfigIssue{{ModelName: minimax, DefaultGroup: "missing", GroupOrder: "missing"}}, coverage.Issues)
	anon := readPricing(0, true)
	assert.NotContains(t, anon, minimax)
	assert.Equal(t, lime, anon[image].DefaultGroup)
	assert.Equal(t, []string{lime}, anon[image].GroupOrder)
	rows := readPricing(member, true)
	require.Len(t, rows, 4)
	for name, order := range expectedOrders {
		assert.Equal(t, order, rows[name].GroupOrder, name)
		want := draft.Models[name].DefaultGroup
		if name == minimax {
			want = bronze
		}
		assert.Equal(t, want, rows[name].DefaultGroup, name)
	}
	observed, _ := check()
	assert.Equal(t, coverage.Fallbacks+1, observed.Fallbacks)

	saved.Models[minimax] = model.PricingGroupEntry{DefaultGroup: bronze, GroupOrder: expectedOrders[minimax]}
	saved.Models[seedance] = model.PricingGroupEntry{DefaultGroup: bronze, GroupOrder: expectedOrders[seedance]}
	put(saved, http.StatusOK, true)
	assert.Empty(t, readPricing(0, true)[seedance].DefaultGroup)
	updated := readPricing(member, true)
	assert.Equal(t, bronze, updated[seedance].DefaultGroup)
	assert.Equal(t, expectedOrders[seedance], updated[seedance].GroupOrder)
	put(saved, http.StatusConflict, false)
	complete, saved := check()
	assert.EqualValues(t, 2, saved.Version)
	assert.Empty(t, complete.Issues)
	verifyOptionAliases(draft)
	_, afterAliases := check()
	assert.Equal(t, saved, afterAliases)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"FixtureOptionA": "synthetic", "FixtureOptionB": "synthetic"}))
	for _, invalid := range []model.PricingGroupEntry{
		{DefaultGroup: "fixture-unknown", GroupOrder: expectedOrders[seedance]},
		{DefaultGroup: bronze, GroupOrder: []string{bronze, bronze}},
		{DefaultGroup: bronze, GroupOrder: []string{bronze, "fixture-unknown"}},
		{DefaultGroup: bronze, GroupOrder: []string{bronze}},
	} {
		saved.Models[seedance] = invalid
		put(saved, http.StatusOK, false)
	}
	assert.Equal(t, bronze, readPricing(member, true)[seedance].DefaultGroup)

	// The release check must not repair malformed channel settings on read.
	const malformedSettings = `{"advanced_custom":`
	advanced := model.Channel{
		Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled,
		Group: lime, Key: "fixture", Models: image, OtherSettings: malformedSettings,
	}
	require.NoError(t, db.Create(&advanced).Error)
	require.NoError(t, advanced.AddAbilities(nil))
	response := call(http.MethodGet, checkPath, root, nil)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	put(afterAliases, http.StatusOK, false)
	var channel model.Channel
	require.NoError(t, db.First(&channel, advanced.Id).Error)
	assert.Equal(t, malformedSettings, channel.OtherSettings)
	require.NoError(t, db.Model(&advanced).Update("settings", "{}").Error)

	rules, err := modelcontracts.ReadVideoCapabilities(strings.NewReader(`{"models":{"seedance-2-5":[]}}`))
	require.NoError(t, err)
	oldRules := modelcontracts.SetVideoCapabilities(rules)
	t.Cleanup(func() { modelcontracts.SetVideoCapabilities(oldRules) })
	checked, stillSaved := check()
	assert.Empty(t, checked.Issues)
	assert.Equal(t, afterAliases, stillSaved)
	if kind == "sqlite" {
		// Even a warm channel cache must not hide a failed video projection read.
		for _, useCache := range []bool{false, true} {
			common.MemoryCacheEnabled = useCache
			model.InitChannelCache()
			model.InvalidatePricingCache()
			require.Len(t, readPricing(member, false), len(expectedOrders))
			require.NoError(t, db.Migrator().RenameColumn(&model.Channel{}, "setting", "fixture_unavailable_setting"))
			response := call(http.MethodGet, checkPath, root, nil)
			assert.Equal(t, http.StatusServiceUnavailable, response.Code, "memory cache %t: %s", useCache, response.Body.String())
			assert.Len(t, readPricing(member, false), len(expectedOrders))
			require.NoError(t, db.Migrator().RenameColumn(&model.Channel{}, "fixture_unavailable_setting", "setting"))
		}
		common.MemoryCacheEnabled = false
	}

	// A fresh catalog snapshot must preserve the configured fields regardless of
	// the iteration order of the legacy enabled-group set.
	model.InvalidatePricingCache()
	for name, row := range readPricing(member, false) {
		assert.Equal(t, updated[name].DefaultGroup, row.DefaultGroup, name)
		assert.Equal(t, updated[name].GroupOrder, row.GroupOrder, name)
	}

	// Remove a group's ability after saving. A stale explicit default stays
	// empty, and the defensive order drops the retired group with the catalog.
	require.NoError(t, db.Model(&model.Ability{}).Where(&model.Ability{Model: seedance, Group: bronze}).Update("enabled", false).Error)
	model.InvalidatePricingCache()
	stale, _ := check()
	assert.Equal(t, []model.PricingGroupConfigIssue{{ModelName: seedance, DefaultGroup: "not_enabled", GroupOrder: "group_set_mismatch"}}, stale.Issues)
	retired := readPricing(member, false)[seedance]
	assert.Equal(t, []string{lime}, retired.GroupOrder)
	assert.Empty(t, retired.DefaultGroup)

	var persisted model.Option
	require.NoError(t, db.Where(&model.Option{Key: model.PricingGroupConfigKey}).Take(&persisted).Error)
	optionQuery := db.Model(&model.Option{}).Where(&model.Option{Key: persisted.Key})
	require.NoError(t, optionQuery.Update("value", `{"version":2}`).Error)
	fallback := readPricing(member, false)
	assert.Equal(t, lime, fallback[seedance].DefaultGroup)
	assert.Equal(t, fallback[wan].EnableGroup, fallback[wan].GroupOrder)
	require.NoError(t, optionQuery.Update("value", persisted.Value).Error)
	observed, _ = check()
	assert.Equal(t, stale.Failures+1, observed.Failures)

	if kind == "sqlite" {
		// A cached public catalog must not turn unavailable source tables
		// into a passing release check, with or without a warmed cache.
		for _, table := range []string{"abilities", "models", "vendors", "options", "channels"} {
			temporary := "fixture_unavailable_" + table
			require.NoError(t, db.Migrator().RenameTable(table, temporary))
			assert.Equal(t, http.StatusServiceUnavailable, call(http.MethodGet, checkPath, root, nil).Code)
			assert.Equal(t, http.StatusOK, call(http.MethodGet, "/api/pricing", member, nil).Code)
			model.InvalidatePricingCache()
			assert.Equal(t, http.StatusServiceUnavailable, call(http.MethodGet, checkPath, root, nil).Code)
			require.NoError(t, db.Migrator().RenameTable(temporary, table))
			readPricing(member, false)
		}
		// A verified empty catalog produces an empty array instead of an error.
		require.NoError(t, db.Model(&model.Ability{}).Where("enabled = ?", true).Update("enabled", false).Error)
		empty, _ := check()
		assert.NotNil(t, empty.Issues)
		assert.Empty(t, empty.Issues)
	}
}
