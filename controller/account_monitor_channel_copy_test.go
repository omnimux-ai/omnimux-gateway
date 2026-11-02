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
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accountMonitorCopyFixture(t *testing.T) model.Channel {
	t.Helper()
	routes := []dto.AdvancedCustomRoute{
		{Models: []string{"x-user"}, UpstreamPath: "/api/v1/twitter/web/fetch_user_profile", QueryMapping: map[string]string{"screen_name": "screen_name", "rest_id": "rest_id"}},
		{Models: []string{"x-posts"}, UpstreamPath: "/api/v1/twitter/web/fetch_user_post_tweet", QueryMapping: map[string]string{"screen_name": "screen_name", "rest_id": "rest_id", "cursor": "cursor"}},
		{Models: []string{"tiktok-user"}, UpstreamPath: "/api/v1/tiktok/web/fetch_user_profile", QueryMapping: map[string]string{"uniqueId": "uniqueId", "secUid": "secUid"}},
		{Models: []string{"tiktok-posts"}, UpstreamPath: "/api/v1/tiktok/app/v3/fetch_user_post_videos", QueryMapping: map[string]string{"unique_id": "unique_id", "sec_user_id": "sec_user_id", "count": "count", "max_cursor": "max_cursor", "sort_type": "sort_type", "region": "region"}},
	}
	models := []string{"x-user", "x-posts", "tiktok-user", "tiktok-posts"}
	for i := range 34 {
		name := fmt.Sprintf("retired-%d", i)
		models = append(models, name)
		routes = append(routes, dto.AdvancedCustomRoute{Models: []string{name}, UpstreamPath: fmt.Sprintf("/api/v1/retired/%d", i)})
	}
	for i := range routes {
		routes[i].IncomingPath = "/v1/chat/completions"
		routes[i].UpstreamMethod = http.MethodGet
		routes[i].Converter = "none"
		routes[i].Auth = &dto.AdvancedCustomRouteAuth{Type: "header", Name: "Authorization", Value: "Bearer fake-route-private-value"}
	}
	channel := model.Channel{
		Type: constant.ChannelTypeAdvancedCustom, Key: "fake-channel-private-value", Status: common.ChannelStatusManuallyDisabled,
		Name: "Legacy data", Models: strings.Join(models, ","), Group: "auto,default", BaseURL: common.GetPointer("https://api.tikhub.io"),
		Tag: common.GetPointer("legacy"), Balance: 12, UsedQuota: 123,
		ModelMapping: common.GetPointer(`{"x-user":"unrelated"}`), ParamOverride: common.GetPointer(`{"secret":"fake-override-private-value"}`),
		HeaderOverride: common.GetPointer(`{"X-Extra":"fake-header-private-value"}`), StatusCodeMapping: common.GetPointer(`{"401":"200"}`),
	}
	channel.SetSetting(dto.ChannelSettings{SystemPrompt: "unrelated instruction", ForceFormat: true})
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		ToolLossPolicy: "strict", UpstreamModelUpdateAutoSyncEnabled: true,
		AdvancedCustom: &dto.AdvancedCustomConfig{Routes: routes, ModelByParam: map[string]dto.ParamModelRouting{"x-user": {Param: "size", Rules: []dto.ParamModelRule{{AnyOf: []string{"large"}, Model: "unrelated"}}}}},
	})
	return channel
}

func performAccountMonitorCopy(t *testing.T, id int, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(id)}}
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	require.NoError(t, model.DB.AutoMigrate(&model.AuditLog{}))
	ctx.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/copy/%d?reset_balance=false&suffix=untrusted", id), strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CopyChannel(ctx)
	return recorder
}

func TestCopyChannelAccountMonitorNarrowsScopeAndKeepsCredentialsServerSide(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	origin := accountMonitorCopyFixture(t)
	require.NoError(t, db.Create(&origin).Error)
	before, err := common.Marshal(origin)
	require.NoError(t, err)
	recorder := performAccountMonitorCopy(t, origin.Id, `{"scope":"account-monitor"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, recorder.Body.String())
	require.Positive(t, response.Data.ID)
	assert.NotContains(t, recorder.Body.String(), "private-value")
	var responseObject map[string]common.RawMessage
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &responseObject))
	var resultObject map[string]common.RawMessage
	require.NoError(t, common.Unmarshal(responseObject["data"], &resultObject))
	assert.Len(t, resultObject, 1)
	var cloned model.Channel
	require.NoError(t, db.First(&cloned, response.Data.ID).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, cloned.Status)
	assert.Equal(t, "social-data-tikhub", cloned.Group)
	assert.Equal(t, "data-api", cloned.GetTag())
	assert.Contains(t, cloned.Name, "账号监控")
	assert.Equal(t, "x-user,x-posts,tiktok-user,tiktok-posts", cloned.Models)
	assert.Equal(t, origin.Key, cloned.Key)
	assert.Nil(t, cloned.ModelMapping)
	assert.Nil(t, cloned.ParamOverride)
	assert.Nil(t, cloned.HeaderOverride)
	assert.Empty(t, cloned.GetStatusCodeMapping())
	assert.Nil(t, cloned.Setting)
	assert.Zero(t, cloned.Balance)
	assert.Zero(t, cloned.UsedQuota)
	var settings dto.ChannelOtherSettings
	require.NoError(t, common.Unmarshal([]byte(cloned.OtherSettings), &settings))
	require.NotNil(t, settings.AdvancedCustom)
	assert.Empty(t, settings.AdvancedCustom.ModelByParam)
	assert.Empty(t, settings.ToolLossPolicy)
	assert.False(t, settings.UpstreamModelUpdateCheckEnabled)
	assert.False(t, settings.UpstreamModelUpdateAutoSyncEnabled)
	require.Len(t, settings.AdvancedCustom.Routes, 4)
	for _, route := range settings.AdvancedCustom.Routes {
		require.Len(t, route.Models, 1)
		assert.Equal(t, "Bearer fake-route-private-value", route.Auth.Value)
		assert.NoError(t, route.ValidateResponseValidation())
		assert.NotEmpty(t, route.ResponseValidation)
		assert.NotContains(t, route.QueryMapping, "sort_type")
		assert.NotContains(t, route.QueryMapping, "region")
	}
	var abilities []model.Ability
	require.NoError(t, db.Where("channel_id = ?", cloned.Id).Find(&abilities).Error)
	require.Len(t, abilities, 4)
	for _, ability := range abilities {
		assert.False(t, ability.Enabled)
		assert.Equal(t, cloned.Group, ability.Group)
	}
	var saved model.Channel
	require.NoError(t, db.First(&saved, origin.Id).Error)
	after, err := common.Marshal(saved)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "copy must not change the source")
}

func TestCopyChannelAccountMonitorAcceptsLegacyStoppedStatus(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	origin := accountMonitorCopyFixture(t)
	require.NoError(t, db.Create(&origin).Error)
	require.NoError(t, db.Model(&origin).Update("status", common.ChannelStatusUnknown).Error)
	recorder := performAccountMonitorCopy(t, origin.Id, `{"scope":"account-monitor"}`)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var cloned model.Channel
	require.NoError(t, db.Where("id <> ?", origin.Id).First(&cloned).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, cloned.Status)
	var source model.Channel
	require.NoError(t, db.First(&source, origin.Id).Error)
	assert.Equal(t, common.ChannelStatusUnknown, source.Status)
}

func TestCopyChannelAccountMonitorRejectsUnsafeSourcesWithoutChangingThem(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		modify func(*model.Channel, *dto.ChannelOtherSettings)
	}{
		{"enabled", func(c *model.Channel, _ *dto.ChannelOtherSettings) { c.Status = common.ChannelStatusEnabled }},
		{"unknown status", func(c *model.Channel, _ *dto.ChannelOtherSettings) { c.Status = 99 }},
		{"wrong provider type", func(c *model.Channel, _ *dto.ChannelOtherSettings) { c.Type = constant.ChannelTypeOpenAI }},
		{"unexpected base URL", func(c *model.Channel, _ *dto.ChannelOtherSettings) {
			c.BaseURL = common.GetPointer("https://untrusted.example")
		}},
		{"missing model", func(c *model.Channel, _ *dto.ChannelOtherSettings) {
			c.Models = strings.ReplaceAll(c.Models, "x-user,", "")
		}},
		{"missing route", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes = s.AdvancedCustom.Routes[1:]
		}},
		{"duplicate route", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes = append(s.AdvancedCustom.Routes, s.AdvancedCustom.Routes[0])
		}},
		{"catch all", func(_ *model.Channel, s *dto.ChannelOtherSettings) { s.AdvancedCustom.Routes[4].Models = nil }},
		{"blank catch all", func(_ *model.Channel, s *dto.ChannelOtherSettings) { s.AdvancedCustom.Routes[4].Models = []string{" "} }},
		{"regex model", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[4].Models = []string{"re:^x-"}
		}},
		{"shared model route", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].Models = []string{"x-user", "retired-extra"}
		}},
		{"wrong incoming path", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].IncomingPath = "/v1/responses"
		}},
		{"wrong upstream path", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].UpstreamPath = "/api/v1/unexpected"
		}},
		{"write method", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].UpstreamMethod = http.MethodPost
		}},
		{"converter", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].Converter = "openai_chat_completions_to_anthropic_messages"
		}},
		{"non-passthrough", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].PassthroughResponse = common.GetPointer(false)
		}},
		{"nested identity mapping", func(_ *model.Channel, s *dto.ChannelOtherSettings) {
			s.AdvancedCustom.Routes[0].QueryMapping["screen_name"] = "raw.screen_name"
		}},
		{"malformed transport settings", func(c *model.Channel, _ *dto.ChannelOtherSettings) {
			c.Setting = common.GetPointer("invalid-private-value")
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			origin := accountMonitorCopyFixture(t)
			var settings dto.ChannelOtherSettings
			require.NoError(t, common.Unmarshal([]byte(origin.OtherSettings), &settings))
			scenario.modify(&origin, &settings)
			origin.SetOtherSettings(settings)
			require.NoError(t, db.Create(&origin).Error)
			before, err := common.Marshal(origin)
			require.NoError(t, err)
			recorder := performAccountMonitorCopy(t, origin.Id, `{"scope":"account-monitor"}`)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
			assert.NotContains(t, recorder.Body.String(), "private-value")
			var count int64
			require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
			assert.EqualValues(t, 1, count)
			var saved model.Channel
			require.NoError(t, db.First(&saved, origin.Id).Error)
			after, err := common.Marshal(saved)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
		})
	}
	t.Run("malformed route settings", func(t *testing.T) {
		db := setupModelListControllerTestDB(t)
		origin := accountMonitorCopyFixture(t)
		origin.OtherSettings = "invalid-private-value"
		require.NoError(t, db.Create(&origin).Error)
		recorder := performAccountMonitorCopy(t, origin.Id, `{"scope":"account-monitor"}`)
		assert.Contains(t, recorder.Body.String(), `"success":false`)
		assert.NotContains(t, recorder.Body.String(), "private-value")
		var saved model.Channel
		require.NoError(t, db.First(&saved, origin.Id).Error)
		assert.Equal(t, origin.OtherSettings, saved.OtherSettings)
	})
}

func TestCopyChannelRejectsInvalidScopeRequests(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"scope":null}`, `{"scope":1}`, `{"scope":""}`, `{"scope":"other"}`,
		`{"scope":"account-monitor","key":"private-value"}`, `{"scope":"account-monitor"} {}`, strings.Repeat(" ", 4097),
	} {
		t.Run(body[:min(len(body), 70)], func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			origin := model.Channel{Type: constant.ChannelTypeOpenAI, Name: "legacy", Models: "gpt-test", Group: "default", Key: "fake-private-value"}
			require.NoError(t, db.Create(&origin).Error)
			recorder := performAccountMonitorCopy(t, origin.Id, body)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
			assert.NotContains(t, recorder.Body.String(), "private-value")
			var count int64
			require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
			assert.EqualValues(t, 1, count)
		})
	}
}

func TestCopyChannelWithoutBodyRetainsLegacyCopy(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	origin := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "legacy", Models: "gpt-test", Group: "default", Key: "fake-private-value", Balance: 3, UsedQuota: 4}
	require.NoError(t, db.Create(&origin).Error)
	recorder := performAccountMonitorCopy(t, origin.Id, "")
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var cloned model.Channel
	require.NoError(t, db.Where("id <> ?", origin.Id).First(&cloned).Error)
	assert.Equal(t, origin.Name+"untrusted", cloned.Name)
	assert.Equal(t, origin.Status, cloned.Status)
	assert.Equal(t, origin.Key, cloned.Key)
	assert.Equal(t, origin.Group, cloned.Group)
	assert.Equal(t, origin.Models, cloned.Models)
	assert.Equal(t, origin.Balance, cloned.Balance)
	assert.Equal(t, origin.UsedQuota, cloned.UsedQuota)
}
