package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorShapeDiagnosticIsStrictAndNotForwarded(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		model   string
		input   string
		enabled bool
	}{
		{"profile omitted", "tiktok-user", `"uniqueId":"gethullo"`, false},
		{"profile false", "tiktok-user", `"uniqueId":"gethullo","shape_diagnostic":false`, false},
		{"profile true", "tiktok-user", `"uniqueId":"gethullo","shape_diagnostic": true `, true},
		{"posts omitted", "x-posts", `"screen_name":"higgsfield_ai"`, false},
		{"posts false", "x-posts", `"screen_name":"higgsfield_ai","shape_diagnostic":false`, false},
		{"posts true", "x-posts", `"screen_name":"higgsfield_ai","shape_diagnostic":true`, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			probe, err := parseAccountMonitorProbe(strings.NewReader(`{"model":"` + scenario.model + `",` + scenario.input + `}`))
			require.NoError(t, err)
			assert.Equal(t, scenario.enabled, probe.shapeDiagnostic)
			assert.NotContains(t, probe.fields, "shape_diagnostic")
			assert.NotContains(t, probe.queryFields, "shape_diagnostic")
		})
	}
}

func TestAccountMonitorShapeDiagnosticRejectsInputBeforeOutbound(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	channel := model.Channel{Type: constant.ChannelTypeAdvancedCustom, Key: "secret-saved-key", Name: "Stopped source", Status: common.ChannelStatusManuallyDisabled, Models: "tiktok-user", Group: "default", BaseURL: common.GetPointer(server.URL)}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{IncomingPath: "/v1/chat/completions", UpstreamPath: "/api/v1/tiktok/web/fetch_user_profile", Models: []string{"tiktok-user"}, UpstreamMethod: "GET", QueryMapping: map[string]string{"uniqueId": "uniqueId"}}}}})
	require.NoError(t, db.Create(&channel).Error)
	before, err := common.Marshal(channel)
	require.NoError(t, err)

	for _, scenario := range []struct {
		name  string
		query string
		body  string
	}{
		{"null", "", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":null}`},
		{"number", "", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":1}`},
		{"string", "", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":"true"}`},
		{"object", "", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":{}}`},
		{"array", "", `{"model":"x-posts","screen_name":"higgsfield_ai","shape_diagnostic":[]}`},
		{"x profile true", "", `{"model":"x-user","screen_name":"higgsfield_ai","shape_diagnostic":true}`},
		{"x profile false", "", `{"model":"x-user","screen_name":"higgsfield_ai","shape_diagnostic":false}`},
		{"tiktok posts true", "", `{"model":"tiktok-posts","unique_id":"gethullo","shape_diagnostic":true}`},
		{"tiktok posts false", "", `{"model":"tiktok-posts","unique_id":"gethullo","shape_diagnostic":false}`},
		{"unknown model", "", `{"model":"secret-model","uniqueId":"gethullo","shape_diagnostic":true}`},
		{"arbitrary keys", "", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":true,"keys":["secret-key"]}`},
		{"arbitrary path", "", `{"model":"x-posts","screen_name":"higgsfield_ai","shape_diagnostic":true,"path":"secret-path"}`},
		{"arbitrary depth", "", `{"model":"x-posts","screen_name":"higgsfield_ai","shape_diagnostic":true,"depth":9}`},
		{"arbitrary URL", "", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":true,"url":"https://secret.example"}`},
		{"query only", "?shape_diagnostic=true", `{"model":"tiktok-user","uniqueId":"gethullo"}`},
		{"query with body flag", "?shape_diagnostic=false", `{"model":"tiktok-user","uniqueId":"gethullo","shape_diagnostic":true}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: common.Interface2String(channel.Id)}}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/test/1"+scenario.query, strings.NewReader(scenario.body))
			TestAccountMonitorChannel(c)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), "secret")
			assert.NotContains(t, recorder.Body.String(), "shape_evidence")
		})
	}
	assert.Zero(t, calls)
	var saved model.Channel
	require.NoError(t, db.First(&saved, channel.Id).Error)
	after, err := common.Marshal(saved)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))
}

func TestAccountMonitorShapeDiagnosticCannotEnableLegacyGET(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	prices := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"tiktok-user":0.001}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices)) })
	user := model.User{Username: "legacy-probe-admin", Group: "default", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Empty(t, r.URL.RawQuery)
		assert.Empty(t, r.Header.Get("shape_diagnostic"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"userInfo":{"user":{"nickname":"secret-profile"}}}}`))
	}))
	defer server.Close()
	channel := model.Channel{Type: constant.ChannelTypeAdvancedCustom, Key: "secret-saved-key", Name: "Stopped source", Status: common.ChannelStatusManuallyDisabled, Models: "tiktok-user", Group: "default", BaseURL: common.GetPointer(server.URL)}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{IncomingPath: "/v1/chat/completions", UpstreamPath: "/api/v1/tiktok/web/fetch_user_profile", Models: []string{"tiktok-user"}, UpstreamMethod: "GET", ResponseValidation: dto.AdvancedCustomResponseValidationTikHubUser}}}})
	require.NoError(t, db.Create(&channel).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: common.Interface2String(channel.Id)}}
	c.Set("id", user.Id)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/test/1?model=tiktok-user&shape_diagnostic=true", nil)
	c.Request.Header.Set("shape_diagnostic", "true")
	TestChannel(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, calls)
	assert.NotContains(t, recorder.Body.String(), "shape_evidence")
	assert.NotContains(t, recorder.Body.String(), "secret")
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}
