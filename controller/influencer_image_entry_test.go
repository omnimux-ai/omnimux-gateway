package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInfluencerSunburstRealtimeGenerationCannotBypassDurableEntry(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Images"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":1}`))
	user := model.User{Username: "fixed-realtime-closed", Status: common.UserStatusEnabled, Group: "default", Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "fixture-only", Models: influencerImageModel, Group: influencerImageGroup, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"]}}`}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
	model.InitChannelCache()
	engine := gin.New()
	reached := false
	selected := 0
	engine.GET("/v1/realtime", func(c *gin.Context) {
		c.Set("id", user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	}, middleware.Distribute(), func(c *gin.Context) {
		reached = true
		selected = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
		c.Status(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=gpt-image-2.5-sunburst", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Omnimux-Group", influencerImageGroup)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	t.Logf("observed status=%d reached_generation_handler=%v selected_channel=%d body=%s", recorder.Code, reached, selected, recorder.Body.String())
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code, "unprepared new image model must be rejected before realtime handler/channel selection")
	assert.False(t, reached)
}

func TestInfluencerSunburstOriginalTaskFetchRoutesRemainReadOnly(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "sunburst-read-only", Status: common.UserStatusEnabled, Group: "default", Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	task := model.Task{TaskID: "original-image-task", UserId: user.Id, Status: model.TaskStatusInProgress, Action: ActionInfluencerSheet, Properties: model.Properties{OriginModelName: influencerImageModel}}
	require.NoError(t, db.Create(&task).Error)
	for _, base := range []string{"/v1/videos/", "/v1/video/generations/", "/v1/images/generations/", "/v1/images/tasks/"} {
		t.Run(base, func(t *testing.T) {
			engine := gin.New()
			reached := false
			engine.GET(base+":task_id", func(c *gin.Context) {
				c.Set("id", user.Id)
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
				common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
				common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{influencerImageModel: true})
			}, middleware.Distribute(), func(c *gin.Context) { reached = true; c.Status(http.StatusOK) })
			request := httptest.NewRequest(http.MethodGet, base+task.TaskID, nil)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.True(t, reached)
		})
	}
	var saved model.User
	require.NoError(t, db.First(&saved, user.Id).Error)
	assert.Equal(t, user.Quota, saved.Quota)
	assert.Zero(t, saved.UsedQuota)
	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
