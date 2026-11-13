package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDistributeDB(t *testing.T) {
	t.Helper()
	_ = i18n.Init()
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认","cheap":"特惠","pro":"专业"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"cheap":0.5,"pro":1.5}`))

	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id: 1, Type: constant.ChannelTypeOpenAI, Key: "k1", Status: common.ChannelStatusEnabled,
		Name: "c-cheap", Weight: &weight, Models: "gpt-4o", Group: "cheap", Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "cheap", Model: "gpt-4o", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)

	require.NoError(t, db.Create(&model.Channel{
		Id: 2, Type: constant.ChannelTypeOpenAI, Key: "k2", Status: common.ChannelStatusEnabled,
		Name: "c-default", Weight: &weight, Models: "gpt-4o", Group: "default", Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "default", Model: "gpt-4o", ChannelId: 2, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)

	model.InitChannelCache()

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		_ = setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups)
		_ = ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func setupTestRouterForDistribute(t *testing.T) *gin.Engine {
	setupTestDistributeDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		// Mock token auth context: Token is in "default" group
		common.SetContextKey(c, constant.ContextKeyUserId, 1)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	})

	r.Use(Distribute())

	r.POST("/v1/chat/completions", func(c *gin.Context) {
		usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		origModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
		channelId, _ := common.GetContextKey(c, constant.ContextKeyChannelId)
		c.JSON(http.StatusOK, gin.H{
			"using_group":    usingGroup,
			"token_group":    tokenGroup,
			"original_model": origModel,
			"channel_id":     channelId,
		})
	})

	return r
}

func TestPerRequestGroupViaHeader(t *testing.T) {
	r := setupTestRouterForDistribute(t)

	// 1. Header: X-Omnimux-Group: cheap
	reqBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Omnimux-Group", "cheap")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "cheap", resp["using_group"])
	assert.Equal(t, "cheap", resp["token_group"])
	assert.Equal(t, "gpt-4o", resp["original_model"])
	assert.Equal(t, float64(1), resp["channel_id"], "Channel 1 is cheap channel")

	// 2. Header: New-Api-Group: default
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("New-Api-Group", "default")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	resp = nil
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "default", resp["using_group"])
	assert.Equal(t, float64(2), resp["channel_id"], "Channel 2 is default channel")
}

func TestPerRequestGroupViaModelSuffix(t *testing.T) {
	r := setupTestRouterForDistribute(t)

	// Model with @group syntax: gpt-4o@cheap
	reqBody := []byte(`{"model":"gpt-4o@cheap","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "cheap", resp["using_group"], "using_group must be set to cheap")
	assert.Equal(t, "cheap", resp["token_group"])
	assert.Equal(t, "gpt-4o", resp["original_model"], "original_model must have @cheap stripped")
	assert.Equal(t, float64(1), resp["channel_id"], "Must route to cheap channel 1")
}

func TestPerRequestGroupUnauthorizedRejected(t *testing.T) {
	r := setupTestRouterForDistribute(t)

	// Attempting to use a group not in user's usable groups: forbidden-vip
	reqBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Omnimux-Group", "forbidden-vip")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "Unauthorized group must return 403")
}

func TestPerRequestGroupDeprecatedRejected(t *testing.T) {
	r := setupTestRouterForDistribute(t)

	// Group in usable groups but deprecated from GroupRatio
	_ = setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认","deprecated_group":"已废弃"}`)

	reqBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Omnimux-Group", "deprecated_group")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "Deprecated group must return 403")
}
